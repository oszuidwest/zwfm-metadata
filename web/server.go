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
)

const cacheControlNoCache = "public, max-age=0, must-revalidate"

// Server provides the HTTP dashboard, API endpoints, and WebSocket connections.
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

// OutputStatus holds output configuration and state for the dashboard API.
type OutputStatus struct {
	Name         string   `json:"name"`
	Type         string   `json:"type"`
	Delay        int      `json:"delay"`
	Inputs       []string `json:"inputs"`
	Formatters   []string `json:"formatters"`
	CurrentInput string   `json:"currentInput,omitzero"`
}

// NewServer initializes the server with pre-generated favicons and a dashboard WebSocket hub.
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

	s.dashboardHub.SetOnConnect(func(conn *utils.WebSocketConn) any {
		return s.getDashboardData()
	})

	go s.startPeriodicDashboardUpdates()

	return s, nil
}

// Start launches the HTTP server and blocks until context cancellation.
func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /favicon.ico", s.faviconHandler)
	mux.HandleFunc("GET /favicon-dark.ico", s.faviconDarkHandler)
	mux.HandleFunc("GET /icon.svg", s.iconSVGHandler)
	mux.HandleFunc("GET /icon-dark.svg", s.iconSVGDarkHandler)
	mux.HandleFunc("GET /apple-touch-icon.png", s.appleTouchIconHandler)
	mux.HandleFunc("GET /apple-touch-icon-dark.png", s.appleTouchIconDarkHandler)
	mux.HandleFunc("GET /{$}", s.dashboardHandler)
	mux.HandleFunc("GET /input/dynamic", s.dynamicInputHandler)
	mux.HandleFunc("GET /ws/dashboard", s.dashboardHub.HandleConnection)

	s.registerOutputRoutes(mux)

	s.server = &http.Server{
		Addr:              ":" + strconv.Itoa(s.port),
		Handler:           s.noIndexMiddleware(mux),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	slog.Info("Starting web server", "port", s.port)

	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("HTTP server encountered an error", "error", err)
		}
	}()

	<-ctx.Done()

	slog.Info("Shutting down web server")
	return s.server.Shutdown(context.Background())
}

// noIndexMiddleware adds headers to prevent search engine indexing.
func (s *Server) noIndexMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("X-Robots-Tag", "noindex, nofollow, noarchive, nosnippet, noimageindex")
		next.ServeHTTP(w, req)
	})
}

// dynamicInputHandler accepts metadata updates via HTTP GET parameters.
func (s *Server) dynamicInputHandler(w http.ResponseWriter, req *http.Request) {
	inputName := req.URL.Query().Get("input")
	title := req.URL.Query().Get("title")
	artist := req.URL.Query().Get("artist")
	songID := req.URL.Query().Get("songID")
	duration := req.URL.Query().Get("duration")
	secret := req.URL.Query().Get("secret")

	if inputName == "" {
		http.Error(w, "Missing required parameter: input", http.StatusBadRequest)
		return
	}

	input, exists := s.router.GetInput(inputName)
	if !exists {
		http.Error(w, fmt.Sprintf("Input '%s' not found", inputName), http.StatusNotFound)
		return
	}

	dynamicInput, ok := input.(*inputs.DynamicInput)
	if !ok {
		http.Error(w, fmt.Sprintf("Input '%s' is not a dynamic input", inputName), http.StatusBadRequest)
		return
	}

	err := dynamicInput.UpdateMetadata(songID, artist, title, duration, secret)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	if _, err := fmt.Fprintf(w, "OK"); err != nil {
		slog.Warn("Failed to write HTTP response", "error", err)
	}
}

// dashboardHandler serves the HTML dashboard.
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
	w.Header().Set("Cache-Control", cacheControlNoCache)

	if _, err := w.Write(s.faviconICO); err != nil {
		slog.Warn("Failed to write favicon.ico response", "error", err)
	}
}

// faviconDarkHandler serves the dark mode variant of the ICO favicon.
func (s *Server) faviconDarkHandler(w http.ResponseWriter, _ *http.Request) {
	if len(s.darkFaviconICO) == 0 {
		http.Error(w, "favicon not available", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "image/x-icon")
	w.Header().Set("Cache-Control", cacheControlNoCache)

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
	w.Header().Set("Cache-Control", cacheControlNoCache)

	if _, err := w.Write(s.iconSVG); err != nil {
		slog.Warn("Failed to write icon.svg response", "error", err)
	}
}

// iconSVGDarkHandler serves the dark mode variant of the SVG icon.
func (s *Server) iconSVGDarkHandler(w http.ResponseWriter, _ *http.Request) {
	if len(s.darkIconSVG) == 0 {
		http.Error(w, "icon not available", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Cache-Control", cacheControlNoCache)

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
	w.Header().Set("Cache-Control", cacheControlNoCache)

	if _, err := w.Write(s.appleIconPNG); err != nil {
		slog.Warn("Failed to write apple-touch-icon.png response", "error", err)
	}
}

// appleTouchIconDarkHandler serves the dark mode variant of the Apple touch icon.
func (s *Server) appleTouchIconDarkHandler(w http.ResponseWriter, _ *http.Request) {
	if len(s.darkAppleIconPNG) == 0 {
		http.Error(w, "apple touch icon not available", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", cacheControlNoCache)

	if _, err := w.Write(s.darkAppleIconPNG); err != nil {
		slog.Warn("Failed to write apple-touch-icon-dark.png response", "error", err)
	}
}

// registerOutputRoutes adds HTTP handlers from outputs implementing RouteRegistrar.
func (s *Server) registerOutputRoutes(mux *http.ServeMux) {
	outputs := s.router.GetOutputs()
	for _, output := range outputs {
		if routeRegistrar, ok := output.(core.RouteRegistrar); ok {
			routeRegistrar.RegisterRoutes(mux)
		}
	}
}

// getDashboardData builds the input/output status payload for WebSocket broadcasts.
func (s *Server) getDashboardData() any {
	inputStatuses := s.router.GetInputStatus()

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

		currentInput := s.router.GetCurrentInputForOutput(output.GetName())
		if currentInput != "" {
			outputStatus.CurrentInput = currentInput
			activeFlows++
		}

		outputStatuses = append(outputStatuses, outputStatus)
	}

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

// startPeriodicDashboardUpdates broadcasts status to all connected dashboard clients.
func (s *Server) startPeriodicDashboardUpdates() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		data := s.getDashboardData()
		s.dashboardHub.Broadcast(data)
	}
}
