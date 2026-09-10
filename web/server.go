// Package web provides HTTP server functionality including a dashboard interface,
// REST API endpoints, and WebSocket connections for real-time updates.
package web

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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
	port          int
	router        *core.MetadataRouter
	server        *http.Server
	dashboardHub  *utils.WebSocketHub
	dashboardPage []byte
	assets        map[string]asset // URL path -> pre-generated icon
}

// dashboardData is the payload pushed to dashboard WebSocket clients.
type dashboardData struct {
	Inputs  []core.InputStatus  `json:"inputs"`
	Outputs []core.OutputStatus `json:"outputs"`
}

// NewServer initializes the server with pre-generated icons and a dashboard WebSocket hub.
func NewServer(port int, router *core.MetadataRouter, stationName, brandColor string) (*Server, error) {
	assets, err := iconAssets(brandColor)
	if err != nil {
		return nil, err
	}

	s := &Server{
		port:          port,
		router:        router,
		dashboardHub:  utils.NewWebSocketHub("dashboard"),
		dashboardPage: []byte(dashboardHTML(stationName, brandColor, utils.Version, utils.GetBuildYear())),
		assets:        assets,
	}

	s.dashboardHub.SetOnConnect(func() any {
		return s.getDashboardData()
	})

	return s, nil
}

// Start launches the HTTP server and blocks until context cancellation.
func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()

	for path, a := range s.assets {
		mux.HandleFunc("GET "+path, serveAsset(a))
	}
	mux.HandleFunc("GET /{$}", s.dashboardHandler)
	mux.HandleFunc("GET /input/dynamic", s.dynamicInputHandler)
	mux.HandleFunc("GET /ws/dashboard", s.dashboardHub.HandleConnection)

	for _, output := range s.router.GetOutputs() {
		if routeRegistrar, ok := output.(core.RouteRegistrar); ok {
			routeRegistrar.RegisterRoutes(mux)
		}
	}

	go s.startPeriodicDashboardUpdates(ctx)

	s.server = &http.Server{
		Addr:              ":" + strconv.Itoa(s.port),
		Handler:           noIndexMiddleware(mux),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	slog.Info("Starting web server", "port", s.port)

	go func() {
		if err := s.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("HTTP server encountered an error", "error", err)
		}
	}()

	<-ctx.Done()

	slog.Info("Shutting down web server")
	return s.server.Shutdown(context.Background())
}

// noIndexMiddleware adds headers to prevent search engine indexing.
func noIndexMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("X-Robots-Tag", "noindex, nofollow, noarchive, nosnippet, noimageindex")
		next.ServeHTTP(w, req)
	})
}

// dynamicInputHandler accepts metadata updates via HTTP GET parameters.
func (s *Server) dynamicInputHandler(w http.ResponseWriter, req *http.Request) {
	query := req.URL.Query()
	inputName := query.Get("input")

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
		SongID:   query.Get("songID"),
		Artist:   query.Get("artist"),
		Title:    query.Get("title"),
		Duration: query.Get("duration"),
		Secret:   query.Get("secret"),
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
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
func serveAsset(a asset) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", a.contentType)
		w.Header().Set("Cache-Control", cacheControlNoCache)

		if _, err := w.Write(a.data); err != nil {
			slog.Warn("Failed to write asset response", "content_type", a.contentType, "error", err)
		}
	}
}

// getDashboardData builds the input/output status payload for WebSocket clients.
func (s *Server) getDashboardData() any {
	return dashboardData{
		Inputs:  s.router.GetInputStatus(),
		Outputs: s.router.GetOutputStatus(),
	}
}

// startPeriodicDashboardUpdates checks the status every second and broadcasts it to
// connected dashboard clients when it differs from the last broadcast.
func (s *Server) startPeriodicDashboardUpdates(ctx context.Context) {
	ticks := time.Tick(1 * time.Second)

	var lastSent []byte
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticks:
			if s.dashboardHub.ClientCount() == 0 {
				continue
			}

			msg, err := json.Marshal(s.getDashboardData())
			if err != nil {
				slog.Warn("Failed to marshal dashboard data", "error", err)
				continue
			}
			if bytes.Equal(msg, lastSent) {
				continue
			}
			lastSent = msg
			s.dashboardHub.BroadcastMessage(msg)
		}
	}
}
