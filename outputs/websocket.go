package outputs

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"
	"zwfm-metadata/config"
	"zwfm-metadata/core"
	"zwfm-metadata/utils"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
)

// WebSocketOutput handles broadcasting metadata to WebSocket clients
type WebSocketOutput struct {
	*core.OutputBase
	core.PassiveComponent
	settings        config.WebSocketOutputConfig
	upgrader        websocket.Upgrader
	clients         map[*websocket.Conn]bool
	mu              sync.RWMutex
	currentMetadata *utils.UniversalMetadata
	metadataMu      sync.RWMutex
	payloadMapper   *utils.PayloadMapper
}

// NewWebSocketOutput creates a new WebSocket output
func NewWebSocketOutput(name string, settings config.WebSocketOutputConfig) *WebSocketOutput {
	var mapper *utils.PayloadMapper
	if settings.PayloadMapping != nil {
		mapper = utils.NewPayloadMapper(settings.PayloadMapping)
	}

	output := &WebSocketOutput{
		OutputBase: core.NewOutputBase(name),
		settings:   settings,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(_ *http.Request) bool {
				// Allow connections from any origin
				// In production, you might want to be more restrictive
				return true
			},
		},
		clients:       make(map[*websocket.Conn]bool),
		payloadMapper: mapper,
	}
	output.SetDelay(settings.Delay)
	return output
}

// RegisterRoutes implements the RouteRegistrar interface
func (w *WebSocketOutput) RegisterRoutes(router *mux.Router) {
	router.HandleFunc(w.settings.Path, w.handleWebSocket).Methods("GET")
	slog.Info("WebSocket route registered", "output", w.GetName(), "path", w.settings.Path)
}

// SendFormattedMetadata implements the Output interface
func (w *WebSocketOutput) SendFormattedMetadata(formattedText string) {
	// Check if value changed to avoid unnecessary broadcasts
	if !w.HasChanged(formattedText) {
		return
	}

	// Create message with basic metadata
	minimalMetadata := &core.Metadata{
		Title:     formattedText, // Use formatted text as title fallback
		UpdatedAt: time.Now(),
	}
	msg := utils.ConvertMetadataWithType(formattedText, minimalMetadata, "metadata_update", "", "")

	// Store current metadata
	w.storeCurrentMetadata(msg)
	w.broadcastMessage(*msg)
}

// SendEnhancedMetadata implements the EnhancedOutput interface
func (w *WebSocketOutput) SendEnhancedMetadata(formattedText string, metadata *core.Metadata, inputName, inputType string) {
	// Check if value changed to avoid unnecessary broadcasts
	if !w.HasChanged(formattedText) {
		return
	}

	// Create message with full metadata
	msg := utils.ConvertMetadataWithType(formattedText, metadata, "metadata_update", inputName, inputType)

	// Store current metadata
	w.storeCurrentMetadata(msg)
	w.broadcastMessage(*msg)
}

// handleWebSocket handles WebSocket connection upgrades
func (w *WebSocketOutput) handleWebSocket(writer http.ResponseWriter, req *http.Request) {
	// Upgrade HTTP connection to WebSocket
	conn, err := w.upgrader.Upgrade(writer, req, nil)
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

	slog.Debug("WebSocket client connected", "remote_addr", req.RemoteAddr, "total_clients", clientCount)

	// Send current metadata if available
	if currentMsg := w.getCurrentMetadata(); currentMsg != nil {
		// Send as metadata update message
		updateMsg := *currentMsg
		updateMsg.Type = "metadata_update"

		var payloadToSend interface{}
		// If custom payload mapping is defined, use it
		if w.payloadMapper != nil {
			// Convert to template data
			templateData := updateMsg.ToTemplateData()
			payloadToSend = w.payloadMapper.MapPayload(templateData)
		} else {
			// Use default message structure
			payloadToSend = updateMsg
		}

		if err := conn.WriteJSON(payloadToSend); err != nil {
			slog.Debug("Failed to send initial metadata message", "error", err)
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

			slog.Debug("WebSocket client disconnected", "remote_addr", req.RemoteAddr, "total_clients", clientCount)
			break
		}
	}
}

// broadcastMessage sends a message to all connected WebSocket clients
func (w *WebSocketOutput) broadcastMessage(msg utils.UniversalMetadata) {
	var payloadToSend interface{}

	// If custom payload mapping is defined, use it
	if w.payloadMapper != nil {
		// Convert to template data
		templateData := msg.ToTemplateData()
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
func (w *WebSocketOutput) storeCurrentMetadata(msg *utils.UniversalMetadata) {
	w.metadataMu.Lock()
	defer w.metadataMu.Unlock()
	w.currentMetadata = msg
}

// getCurrentMetadata retrieves the current metadata for new clients
func (w *WebSocketOutput) getCurrentMetadata() *utils.UniversalMetadata {
	w.metadataMu.RLock()
	defer w.metadataMu.RUnlock()
	if w.currentMetadata == nil {
		return nil
	}
	// Return a copy to avoid race conditions
	msg := *w.currentMetadata
	return &msg
}
