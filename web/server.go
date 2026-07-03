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
	"zwfm-metadata/utils"
)

const cacheControlNoCache = "public, max-age=0, must-revalidate"

// metadataUpdater is satisfied by inputs that accept metadata updates via the HTTP API.
type metadataUpdater interface {
	UpdateMetadata(update *core.MetadataRequest) error
}

// Server provides the HTTP dashboard, API endpoints, and WebSocket connections.
type Server struct {
	port             int
	router           *core.MetadataRouter
	server           *http.Server
	dashboardHub     *utils.WebSocketHub
	dashboardPage    []byte
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
		dashboardHub:     utils.NewWebSocketHub("dashboard"),
		dashboardPage:    []byte(dashboardHTML(stationName, brandColor, utils.Version, utils.GetBuildYear())),
		faviconICO:       faviconICO,
		iconSVG:          []byte(buildHubSVG(brandColor)),
		appleIconPNG:     appleIconPNG,
		darkFaviconICO:   darkFaviconICO,
		darkIconSVG:      []byte(buildHubSVGDark(brandColor)),
		darkAppleIconPNG: darkAppleIconPNG,
	}

	s.dashboardHub.SetOnConnect(func() any {
		return s.getDashboardData()
	})

	return s, nil
}

// Start launches the HTTP server and blocks until context cancellation.
func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /favicon.ico", serveAsset(s.faviconICO, "image/x-icon"))
	mux.HandleFunc("GET /favicon-dark.ico", serveAsset(s.darkFaviconICO, "image/x-icon"))
	mux.HandleFunc("GET /icon.svg", serveAsset(s.iconSVG, "image/svg+xml"))
	mux.HandleFunc("GET /icon-dark.svg", serveAsset(s.darkIconSVG, "image/svg+xml"))
	mux.HandleFunc("GET /apple-touch-icon.png", serveAsset(s.appleIconPNG, "image/png"))
	mux.HandleFunc("GET /apple-touch-icon-dark.png", serveAsset(s.darkAppleIconPNG, "image/png"))
	mux.HandleFunc("GET /{$}", s.dashboardHandler)
	mux.HandleFunc("GET /input/dynamic", s.dynamicInputHandler)
	mux.HandleFunc("GET /ws/dashboard", s.dashboardHub.HandleConnection)

	s.registerOutputRoutes(mux)

	go s.startPeriodicDashboardUpdates(ctx)

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

	updater, ok := input.(metadataUpdater)
	if !ok {
		http.Error(w, fmt.Sprintf("Input '%s' is not a dynamic input", inputName), http.StatusBadRequest)
		return
	}

	err := updater.UpdateMetadata(&core.MetadataRequest{
		SongID:   songID,
		Artist:   artist,
		Title:    title,
		Duration: duration,
		Secret:   secret,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte("OK")); err != nil {
		slog.Warn("Failed to write HTTP response", "error", err)
	}
}

// dashboardHandler serves the HTML dashboard.
func (s *Server) dashboardHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if _, err := w.Write(s.dashboardPage); err != nil {
		slog.Error("Failed to write dashboard HTML response", "error", err)
	}
}

// serveAsset returns a handler that serves a pre-generated static asset.
func serveAsset(data []byte, contentType string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Cache-Control", cacheControlNoCache)

		if _, err := w.Write(data); err != nil {
			slog.Warn("Failed to write asset response", "content_type", contentType, "error", err)
		}
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

// startPeriodicDashboardUpdates broadcasts status to all connected dashboard clients
// until context cancellation. Idle ticks (no connected clients) skip the payload build.
func (s *Server) startPeriodicDashboardUpdates(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if s.dashboardHub.ClientCount() == 0 {
				continue
			}
			s.dashboardHub.Broadcast(s.getDashboardData())
		}
	}
}
