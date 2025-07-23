package web

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"
	"zwfm-metadata/core"
	"zwfm-metadata/inputs"
	"zwfm-metadata/utils"

	"github.com/gorilla/mux"
)

// Server represents the HTTP server
type Server struct {
	port         int
	router       *core.MetadataRouter
	server       *http.Server
	stationName  string
	brandColor   string
	dashboardHub *utils.WebSocketHub
}

// OutputStatus represents the status of an output
type OutputStatus struct {
	Name         string   `json:"name"`
	Type         string   `json:"type"`
	Delay        int      `json:"delay"`
	Inputs       []string `json:"inputs"`
	Formatters   []string `json:"formatters"`
	CurrentInput string   `json:"currentInput,omitempty"`
}

// NewServer creates a new server instance
func NewServer(port int, router *core.MetadataRouter, stationName, brandColor string) *Server {
	s := &Server{
		port:         port,
		router:       router,
		stationName:  stationName,
		brandColor:   brandColor,
		dashboardHub: utils.NewWebSocketHub("dashboard"),
	}

	// Set up dashboard WebSocket callbacks
	s.dashboardHub.SetOnConnect(func(conn *utils.WebSocketConn) interface{} {
		// Send initial dashboard state
		return s.getDashboardData()
	})

	// Start periodic dashboard updates
	go s.periodicDashboardUpdates()

	return s
}

// Start starts the HTTP server
func (s *Server) Start(ctx context.Context) error {
	router := mux.NewRouter()

	// Apply middleware to prevent search engine indexing on all routes
	router.Use(s.noIndexMiddleware)

	// Route handlers
	router.HandleFunc("/", s.dashboardHandler).Methods("GET")
	router.HandleFunc("/status", s.statusHandler).Methods("GET")
	router.HandleFunc("/input/dynamic", s.dynamicInputHandler).Methods("GET")
	router.HandleFunc("/ws/dashboard", s.dashboardWebSocketHandler).Methods("GET")

	// Register WebSocket routes from outputs that implement RouteRegistrar
	s.registerWebSocketRoutes(router)

	// Create HTTP server
	s.server = &http.Server{
		Addr:              ":" + strconv.Itoa(s.port),
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	slog.Info("Starting web server", "port", s.port)

	// Start server in a goroutine
	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("HTTP server encountered an error", "error", err)
		}
	}()

	// Wait for context cancellation
	<-ctx.Done()

	// Shutdown gracefully
	slog.Info("Shutting down web server")
	return s.server.Shutdown(context.Background())
}

// noIndexMiddleware adds headers to prevent search engine indexing on all routes
func (s *Server) noIndexMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("X-Robots-Tag", "noindex, nofollow, noarchive, nosnippet, noimageindex")
		next.ServeHTTP(w, req)
	})
}

// statusHandler handles the /status endpoint - consolidated with dashboard data
func (s *Server) statusHandler(w http.ResponseWriter, req *http.Request) {
	// Use the same handler as dashboard API for consistency
	s.dashboardAPIHandler(w, req)
}

// dynamicInputHandler handles the /input/dynamic endpoint
func (s *Server) dynamicInputHandler(w http.ResponseWriter, req *http.Request) {
	// Get parameters from query string
	inputName := req.URL.Query().Get("input")
	title := req.URL.Query().Get("title")
	artist := req.URL.Query().Get("artist")
	songID := req.URL.Query().Get("songID")
	duration := req.URL.Query().Get("duration")
	secret := req.URL.Query().Get("secret")

	// Validate required parameters
	if inputName == "" {
		http.Error(w, "Missing required parameter: input", http.StatusBadRequest)
		return
	}

	// Get the input
	input, exists := s.router.GetInput(inputName)
	if !exists {
		http.Error(w, fmt.Sprintf("Input '%s' not found", inputName), http.StatusNotFound)
		return
	}

	// Check if it's a dynamic input
	dynamicInput, ok := input.(*inputs.DynamicInput)
	if !ok {
		http.Error(w, fmt.Sprintf("Input '%s' is not a dynamic input", inputName), http.StatusBadRequest)
		return
	}

	// Update the metadata
	err := dynamicInput.UpdateMetadata(songID, artist, title, duration, secret)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Return success response
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	if _, err := fmt.Fprintf(w, "OK"); err != nil {
		slog.Warn("Failed to write HTTP response", "error", err)
	}
}

// dashboardHandler serves the HTML dashboard
func (s *Server) dashboardHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if _, err := w.Write([]byte(dashboardHTML(s.stationName, s.brandColor, utils.Version, utils.GetBuildYear()))); err != nil {
		slog.Error("Failed to write dashboard HTML response", "error", err)
	}
}

// dashboardAPIHandler provides comprehensive JSON data for both dashboard and status endpoint
func (s *Server) dashboardAPIHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Get input statuses
	inputStatuses := s.router.GetInputStatus()

	// Get output information
	outputs := s.router.GetOutputs()
	outputStatuses := make([]OutputStatus, 0, len(outputs))
	activeFlows := 0

	for _, output := range outputs {
		outputStatus := OutputStatus{
			Name:       output.GetName(),
			Type:       s.router.GetOutputType(output.GetName()),
			Delay:      output.GetDelay(),
			Inputs:     s.router.GetOutputInputs(output.GetName()),
			Formatters: s.router.GetOutputFormatterNames(output.GetName()),
		}

		// Get current input for this output
		currentInput := s.router.GetCurrentInputForOutput(output.GetName())
		if currentInput != "" {
			outputStatus.CurrentInput = currentInput
			activeFlows++
		}

		outputStatuses = append(outputStatuses, outputStatus)
	}

	// Create response as anonymous struct
	response := struct {
		Inputs      []core.InputStatus `json:"inputs"`
		Outputs     []OutputStatus     `json:"outputs"`
		ActiveFlows int                `json:"activeFlows"`
	}{
		Inputs:      inputStatuses,
		Outputs:     outputStatuses,
		ActiveFlows: activeFlows,
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

// registerWebSocketRoutes registers WebSocket routes from outputs that implement RouteRegistrar
func (s *Server) registerWebSocketRoutes(router *mux.Router) {
	outputs := s.router.GetOutputs()
	for _, output := range outputs {
		if routeRegistrar, ok := output.(core.RouteRegistrar); ok {
			routeRegistrar.RegisterRoutes(router)
		}
	}
}

// dashboardWebSocketHandler handles WebSocket connections for the dashboard
func (s *Server) dashboardWebSocketHandler(w http.ResponseWriter, r *http.Request) {
	s.dashboardHub.HandleConnection(w, r)
}

// getDashboardData returns the current dashboard data
func (s *Server) getDashboardData() interface{} {
	// Get input statuses
	inputStatuses := s.router.GetInputStatus()

	// Get output information
	outputs := s.router.GetOutputs()
	outputStatuses := make([]OutputStatus, 0, len(outputs))
	activeFlows := 0

	for _, output := range outputs {
		outputStatus := OutputStatus{
			Name:       output.GetName(),
			Type:       s.router.GetOutputType(output.GetName()),
			Delay:      output.GetDelay(),
			Inputs:     s.router.GetOutputInputs(output.GetName()),
			Formatters: s.router.GetOutputFormatterNames(output.GetName()),
		}

		// Get current input for this output
		currentInput := s.router.GetCurrentInputForOutput(output.GetName())
		if currentInput != "" {
			outputStatus.CurrentInput = currentInput
			activeFlows++
		}

		outputStatuses = append(outputStatuses, outputStatus)
	}

	// Return as anonymous struct
	return struct {
		Inputs      []core.InputStatus `json:"inputs"`
		Outputs     []OutputStatus     `json:"outputs"`
		ActiveFlows int                `json:"activeFlows"`
	}{
		Inputs:      inputStatuses,
		Outputs:     outputStatuses,
		ActiveFlows: activeFlows,
	}
}

// periodicDashboardUpdates sends dashboard updates periodically
func (s *Server) periodicDashboardUpdates() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		// Get updated dashboard data and broadcast
		data := s.getDashboardData()
		s.dashboardHub.Broadcast(data)
	}
}
