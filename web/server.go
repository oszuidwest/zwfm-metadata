// Package web provides HTTP server functionality including a dashboard interface,
// REST API endpoints, and WebSocket connections for real-time updates.
package web

import (
	"context"
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

// Server represents the HTTP server.
type Server struct {
	port             int
	router           *core.MetadataRouter
	server           *http.Server
	stationName      string
	brandColor       string
	dashboardHub     *utils.WebSocketHub
	faviconICO       []byte
	iconSVG          []byte
	appleIconPNG     []byte
	darkFaviconICO   []byte
	darkIconSVG      []byte
	darkAppleIconPNG []byte
}

// OutputStatus represents the status of an output.
type OutputStatus struct {
	Name         string   `json:"name"`
	Type         string   `json:"type"`
	Delay        int      `json:"delay"`
	Inputs       []string `json:"inputs"`
	Formatters   []string `json:"formatters"`
	CurrentInput string   `json:"currentInput,omitempty"`
}

// NewServer creates a new server instance.
func NewServer(port int, router *core.MetadataRouter, stationName, brandColor string) (*Server, error) {
	faviconICO, err := generateFaviconICO(brandColor)
	if err != nil {
		return nil, fmt.Errorf("generate favicon.ico: %w", err)
	}

	appleIconPNG, err := generateAppleTouchIconPNG(brandColor)
	if err != nil {
		return nil, fmt.Errorf("generate apple-touch-icon.png: %w", err)
	}

	darkFaviconICO, err := generateFaviconICODark(brandColor)
	if err != nil {
		return nil, fmt.Errorf("generate dark favicon.ico: %w", err)
	}

	darkAppleIconPNG, err := generateAppleTouchIconPNGD(brandColor)
	if err != nil {
		return nil, fmt.Errorf("generate dark apple-touch-icon.png: %w", err)
	}

	s := &Server{
		port:             port,
		router:           router,
		stationName:      stationName,
		brandColor:       brandColor,
		dashboardHub:     utils.NewWebSocketHub("dashboard"),
		faviconICO:       faviconICO,
		iconSVG:          []byte(generateFaviconSVG(brandColor)),
		appleIconPNG:     appleIconPNG,
		darkFaviconICO:   darkFaviconICO,
		darkIconSVG:      []byte(generateFaviconSVGDark(brandColor)),
		darkAppleIconPNG: darkAppleIconPNG,
	}

	// Set up dashboard WebSocket callbacks
	s.dashboardHub.SetOnConnect(func(conn *utils.WebSocketConn) interface{} {
		// Send initial dashboard state
		return s.getDashboardData()
	})

	// Start periodic dashboard updates over WebSocket
	go s.startPeriodicDashboardUpdates()

	return s, nil
}

// Start starts the HTTP server.
func (s *Server) Start(ctx context.Context) error {
	router := mux.NewRouter()

	// Apply middleware to prevent search engine indexing on all routes
	router.Use(s.noIndexMiddleware)

	// Route handlers
	router.HandleFunc("/favicon.ico", s.faviconHandler).Methods("GET")
	router.HandleFunc("/favicon-dark.ico", s.faviconDarkHandler).Methods("GET")
	router.HandleFunc("/icon.svg", s.iconSVGHandler).Methods("GET")
	router.HandleFunc("/icon-dark.svg", s.iconSVGDarkHandler).Methods("GET")
	router.HandleFunc("/apple-touch-icon.png", s.appleTouchIconHandler).Methods("GET")
	router.HandleFunc("/apple-touch-icon-dark.png", s.appleTouchIconDarkHandler).Methods("GET")
	router.HandleFunc("/", s.dashboardHandler).Methods("GET")
	router.HandleFunc("/input/dynamic", s.dynamicInputHandler).Methods("GET")
	router.HandleFunc("/ws/dashboard", s.dashboardHub.HandleConnection).Methods("GET")

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

// dynamicInputHandler handles the /input/dynamic endpoint.
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

// faviconHandler serves ICO favicons with support for legacy browsers.
func (s *Server) faviconHandler(w http.ResponseWriter, _ *http.Request) {
	if len(s.faviconICO) == 0 {
		http.Error(w, "favicon not available", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "image/x-icon")
	w.Header().Set("Cache-Control", "public, max-age=86400")

	if _, err := w.Write(s.faviconICO); err != nil {
		slog.Warn("Failed to write favicon.ico response", "error", err)
	}
}

func (s *Server) faviconDarkHandler(w http.ResponseWriter, _ *http.Request) {
	if len(s.darkFaviconICO) == 0 {
		http.Error(w, "favicon not available", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "image/x-icon")
	w.Header().Set("Cache-Control", "public, max-age=86400")

	if _, err := w.Write(s.darkFaviconICO); err != nil {
		slog.Warn("Failed to write favicon-dark.ico response", "error", err)
	}
}

// iconSVGHandler serves the scalable favicon variant.
func (s *Server) iconSVGHandler(w http.ResponseWriter, _ *http.Request) {
	if len(s.iconSVG) == 0 {
		http.Error(w, "icon not available", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Cache-Control", "public, max-age=86400")

	if _, err := w.Write(s.iconSVG); err != nil {
		slog.Warn("Failed to write icon.svg response", "error", err)
	}
}

func (s *Server) iconSVGDarkHandler(w http.ResponseWriter, _ *http.Request) {
	if len(s.darkIconSVG) == 0 {
		http.Error(w, "icon not available", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Cache-Control", "public, max-age=86400")

	if _, err := w.Write(s.darkIconSVG); err != nil {
		slog.Warn("Failed to write icon-dark.svg response", "error", err)
	}
}

// appleTouchIconHandler serves the Apple touch icon variant.
func (s *Server) appleTouchIconHandler(w http.ResponseWriter, _ *http.Request) {
	if len(s.appleIconPNG) == 0 {
		http.Error(w, "apple touch icon not available", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=86400")

	if _, err := w.Write(s.appleIconPNG); err != nil {
		slog.Warn("Failed to write apple-touch-icon.png response", "error", err)
	}
}

func (s *Server) appleTouchIconDarkHandler(w http.ResponseWriter, _ *http.Request) {
	if len(s.darkAppleIconPNG) == 0 {
		http.Error(w, "apple touch icon not available", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=86400")

	if _, err := w.Write(s.darkAppleIconPNG); err != nil {
		slog.Warn("Failed to write apple-touch-icon-dark.png response", "error", err)
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

// getDashboardData returns the current dashboard data for WebSocket broadcasts.
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

// startPeriodicDashboardUpdates sends dashboard status updates every second over WebSocket
func (s *Server) startPeriodicDashboardUpdates() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		data := s.getDashboardData()
		s.dashboardHub.Broadcast(data)
	}
}
