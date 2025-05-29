package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"zwfm-metadata/core"
	"zwfm-metadata/inputs"
	"zwfm-metadata/utils"

	"github.com/gorilla/mux"
)

// Server represents the HTTP server
type Server struct {
	port    int
	manager *core.Manager
	server  *http.Server
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
func NewServer(port int, manager *core.Manager) *Server {
	return &Server{
		port:    port,
		manager: manager,
	}
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

	// Create HTTP server
	s.server = &http.Server{
		Addr:    ":" + strconv.Itoa(s.port),
		Handler: router,
	}

	utils.LogInfo("Starting web server on port %d", s.port)

	// Start server in a goroutine
	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			utils.LogError("HTTP server encountered an error: %v", err)
		}
	}()

	// Wait for context cancellation
	<-ctx.Done()

	// Shutdown gracefully
	utils.LogInfo("Shutting down web server...")
	return s.server.Shutdown(context.Background())
}

// noIndexMiddleware adds headers to prevent search engine indexing on all routes
func (s *Server) noIndexMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Robots-Tag", "noindex, nofollow, noarchive, nosnippet, noimageindex")
		next.ServeHTTP(w, r)
	})
}

// statusHandler handles the /status endpoint - consolidated with dashboard data
func (s *Server) statusHandler(w http.ResponseWriter, r *http.Request) {
	// Use the same handler as dashboard API for consistency
	s.dashboardAPIHandler(w, r)
}

// dynamicInputHandler handles the /input/dynamic endpoint
func (s *Server) dynamicInputHandler(w http.ResponseWriter, r *http.Request) {
	// Get parameters from query string
	inputName := r.URL.Query().Get("input")
	title := r.URL.Query().Get("title")
	artist := r.URL.Query().Get("artist")
	songID := r.URL.Query().Get("songID")
	duration := r.URL.Query().Get("duration")
	secret := r.URL.Query().Get("secret")

	// Validate required parameters
	if inputName == "" {
		http.Error(w, "Missing required parameter: input", http.StatusBadRequest)
		return
	}

	// Get the input
	input, exists := s.manager.GetInput(inputName)
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
		utils.LogWarn("Failed to write HTTP response: %v", err)
	}
}

// dashboardHandler serves the HTML dashboard
func (s *Server) dashboardHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if _, err := w.Write([]byte(dashboardHTML())); err != nil {
		utils.LogError("Failed to write dashboard HTML response: %v", err)
	}
}

// dashboardAPIHandler provides comprehensive JSON data for both dashboard and status endpoint
func (s *Server) dashboardAPIHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Get input statuses
	inputStatuses := s.manager.GetInputStatus()

	// Get output information
	outputs := s.manager.GetOutputs()
	outputStatuses := make([]OutputStatus, 0, len(outputs))
	activeFlows := 0

	for _, output := range outputs {
		outputStatus := OutputStatus{
			Name:       output.GetName(),
			Type:       s.manager.GetOutputType(output.GetName()),
			Delay:      output.GetDelay(),
			Inputs:     s.manager.GetOutputInputs(output.GetName()),
			Formatters: s.manager.GetOutputFormatterNames(output.GetName()),
		}

		// Get current input for this output
		currentInput := s.manager.GetCurrentInputForOutput(output.GetName())
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
