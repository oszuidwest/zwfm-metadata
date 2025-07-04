package outputs

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"
	"zwfm-metadata/config"
	"zwfm-metadata/core"
	"zwfm-metadata/utils"

	"github.com/gorilla/websocket"
)

// WebSocketOutput handles broadcasting metadata to WebSocket clients
type WebSocketOutput struct {
	*core.OutputBase
	settings        config.WebSocketOutputConfig
	server          *http.Server
	upgrader        websocket.Upgrader
	clients         map[*websocket.Conn]bool
	mu              sync.RWMutex
	currentMetadata *WebSocketMessage
	metadataMu      sync.RWMutex
	payloadMapper   *utils.PayloadMapper
}

// WebSocketMessage represents the message sent to WebSocket clients
type WebSocketMessage struct {
	Type              string     `json:"type"`
	FormattedMetadata string     `json:"formatted_metadata"`
	SongID            string     `json:"songID,omitempty"`
	Title             string     `json:"title"`
	Artist            string     `json:"artist,omitempty"`
	Duration          string     `json:"duration,omitempty"`
	UpdatedAt         time.Time  `json:"updated_at"`
	ExpiresAt         *time.Time `json:"expires_at,omitempty"`
}

// NewWebSocketOutput creates a new WebSocket output
func NewWebSocketOutput(name string, settings config.WebSocketOutputConfig) *WebSocketOutput {
	var mapper *utils.PayloadMapper
	if settings.PayloadMapping != nil {
		mapper = utils.NewPayloadMapper(settings.PayloadMapping)
	}

	return &WebSocketOutput{
		OutputBase: core.NewOutputBase(name),
		settings:   settings,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				// Allow connections from any origin
				// In production, you might want to be more restrictive
				return true
			},
		},
		clients:       make(map[*websocket.Conn]bool),
		payloadMapper: mapper,
	}
}

// Start implements the Output interface
func (w *WebSocketOutput) Start(ctx context.Context) error {
	// Create HTTP server mux
	mux := http.NewServeMux()
	mux.HandleFunc(w.settings.Path, w.handleWebSocket)

	// Create server
	w.server = &http.Server{
		Addr:    w.settings.Address,
		Handler: mux,
	}

	// Start server in goroutine
	go func() {
		slog.Info("WebSocket server starting", "address", w.settings.Address, "path", w.settings.Path)
		if err := w.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("WebSocket server error", "error", err)
		}
	}()

	// Wait for context cancellation
	<-ctx.Done()

	// Shutdown server gracefully
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Close all client connections
	w.mu.Lock()
	for client := range w.clients {
		if err := client.Close(); err != nil {
			slog.Debug("Failed to close client connection", "error", err)
		}
	}
	w.clients = make(map[*websocket.Conn]bool)
	w.mu.Unlock()

	return w.server.Shutdown(shutdownCtx)
}

// GetDelay implements the Output interface
func (w *WebSocketOutput) GetDelay() int {
	return w.settings.Delay
}

// SendFormattedMetadata implements the Output interface
func (w *WebSocketOutput) SendFormattedMetadata(formattedText string) {
	// Check if value changed to avoid unnecessary broadcasts
	if !w.HasChanged(formattedText) {
		return
	}

	// Create message with basic metadata
	msg := WebSocketMessage{
		Type:              "metadata_update",
		FormattedMetadata: formattedText,
		Title:             formattedText, // Use formatted text as title fallback
		UpdatedAt:         time.Now(),
	}

	// Store current metadata
	w.storeCurrentMetadata(&msg)
	w.broadcastMessage(msg)
}

// SendEnhancedMetadata implements the EnhancedOutput interface
func (w *WebSocketOutput) SendEnhancedMetadata(formattedText string, metadata *core.Metadata) {
	// Check if value changed to avoid unnecessary broadcasts
	if !w.HasChanged(formattedText) {
		return
	}

	// Create message with full metadata
	msg := WebSocketMessage{
		Type:              "metadata_update",
		FormattedMetadata: formattedText,
		SongID:            metadata.SongID,
		Title:             metadata.Title,
		Artist:            metadata.Artist,
		Duration:          metadata.Duration,
		UpdatedAt:         metadata.UpdatedAt,
		ExpiresAt:         metadata.ExpiresAt,
	}

	// Store current metadata
	w.storeCurrentMetadata(&msg)
	w.broadcastMessage(msg)
}

// handleWebSocket handles WebSocket connection upgrades
func (w *WebSocketOutput) handleWebSocket(rw http.ResponseWriter, r *http.Request) {
	// Upgrade HTTP connection to WebSocket
	conn, err := w.upgrader.Upgrade(rw, r, nil)
	if err != nil {
		slog.Error("Failed to upgrade WebSocket connection", "error", err)
		return
	}
	defer func() {
		if err := conn.Close(); err != nil {
			slog.Debug("Failed to close WebSocket connection", "error", err)
		}
	}()

	// Add client to the list
	w.mu.Lock()
	w.clients[conn] = true
	clientCount := len(w.clients)
	w.mu.Unlock()

	slog.Debug("WebSocket client connected", "remote_addr", r.RemoteAddr, "total_clients", clientCount)

	// Send current metadata if available
	if currentMsg := w.getCurrentMetadata(); currentMsg != nil {
		// Send as welcome message
		welcomeMsg := *currentMsg
		welcomeMsg.Type = "welcome"

		var payloadToSend interface{}
		// If custom payload mapping is defined, use it
		if w.payloadMapper != nil {
			// Convert to template data
			templateData := w.messageToTemplateData(welcomeMsg)
			payloadToSend = w.payloadMapper.MapPayload(templateData)
		} else {
			// Use default message structure
			payloadToSend = welcomeMsg
		}

		if err := conn.WriteJSON(payloadToSend); err != nil {
			slog.Debug("Failed to send welcome message", "error", err)
		}
	}

	// Keep connection alive and handle ping/pong
	if err := conn.SetReadDeadline(time.Now().Add(60 * time.Second)); err != nil {
		slog.Debug("Failed to set read deadline", "error", err)
	}
	conn.SetPongHandler(func(string) error {
		if err := conn.SetReadDeadline(time.Now().Add(60 * time.Second)); err != nil {
			slog.Debug("Failed to set read deadline in pong handler", "error", err)
		}
		return nil
	})

	// Start ping ticker
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// Handle client messages (mainly for ping/pong)
	go func() {
		for range ticker.C {
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}()

	// Read messages from client (to detect disconnection)
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			// Client disconnected
			w.mu.Lock()
			delete(w.clients, conn)
			clientCount := len(w.clients)
			w.mu.Unlock()

			slog.Debug("WebSocket client disconnected", "remote_addr", r.RemoteAddr, "total_clients", clientCount)
			break
		}
	}
}

// broadcastMessage sends a message to all connected WebSocket clients
func (w *WebSocketOutput) broadcastMessage(msg WebSocketMessage) {
	var payloadToSend interface{}

	// If custom payload mapping is defined, use it
	if w.payloadMapper != nil {
		// Convert to template data
		templateData := w.messageToTemplateData(msg)
		payloadToSend = w.payloadMapper.MapPayload(templateData)
	} else {
		// Use default message structure
		payloadToSend = msg
	}

	// Marshal message to JSON
	jsonData, err := json.Marshal(payloadToSend)
	if err != nil {
		slog.Error("Failed to marshal WebSocket message", "error", err)
		return
	}

	// Send to all connected clients
	w.mu.RLock()
	clients := make([]*websocket.Conn, 0, len(w.clients))
	for client := range w.clients {
		clients = append(clients, client)
	}
	w.mu.RUnlock()

	// Send message to each client
	for _, client := range clients {
		if err := client.WriteMessage(websocket.TextMessage, jsonData); err != nil {
			// Remove disconnected client
			w.mu.Lock()
			delete(w.clients, client)
			w.mu.Unlock()
			if closeErr := client.Close(); closeErr != nil {
				slog.Debug("Failed to close disconnected client", "error", closeErr)
			}
			slog.Debug("Removed disconnected WebSocket client", "error", err)
		}
	}

	slog.Debug("Broadcasted WebSocket message", "type", msg.Type, "clients", len(clients))
}

// storeCurrentMetadata stores the current metadata for new clients
func (w *WebSocketOutput) storeCurrentMetadata(msg *WebSocketMessage) {
	w.metadataMu.Lock()
	defer w.metadataMu.Unlock()
	w.currentMetadata = msg
}

// getCurrentMetadata retrieves the current metadata for new clients
func (w *WebSocketOutput) getCurrentMetadata() *WebSocketMessage {
	w.metadataMu.RLock()
	defer w.metadataMu.RUnlock()
	if w.currentMetadata == nil {
		return nil
	}
	// Return a copy to avoid race conditions
	msg := *w.currentMetadata
	return &msg
}

// messageToTemplateData converts a WebSocketMessage to template data
func (w *WebSocketOutput) messageToTemplateData(msg WebSocketMessage) map[string]interface{} {
	data := map[string]interface{}{
		"type":               msg.Type,
		"formatted_metadata": msg.FormattedMetadata,
		"songID":             msg.SongID,
		"title":              msg.Title,
		"artist":             msg.Artist,
		"duration":           msg.Duration,
		"updated_at":         msg.UpdatedAt.Format(time.RFC3339),
	}

	if msg.ExpiresAt != nil {
		data["expires_at"] = msg.ExpiresAt.Format(time.RFC3339)
	} else {
		data["expires_at"] = ""
	}

	return data
}
