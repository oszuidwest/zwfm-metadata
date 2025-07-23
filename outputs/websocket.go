package outputs

import (
	"fmt"
	"log/slog"
	"sync"
	"time"
	"zwfm-metadata/config"
	"zwfm-metadata/core"
	"zwfm-metadata/utils"

	"github.com/gorilla/mux"
)

// WebSocketOutput handles broadcasting metadata to WebSocket clients
type WebSocketOutput struct {
	*core.OutputBase
	core.PassiveComponent
	settings        config.WebSocketOutputConfig
	hub             *utils.WebSocketHub
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
		OutputBase:    core.NewOutputBase(name),
		settings:      settings,
		hub:           utils.NewWebSocketHub(name),
		payloadMapper: mapper,
	}
	output.SetDelay(settings.Delay)

	// Set up WebSocket callbacks
	output.hub.SetOnConnect(func(conn *utils.WebSocketConn) interface{} {
		// Send current metadata to new connections
		output.metadataMu.RLock()
		defer output.metadataMu.RUnlock()
		if output.currentMetadata != nil {
			return output.preparePayload(*output.currentMetadata)
		}
		return nil
	})

	return output
}

// RegisterRoutes implements the RouteRegistrar interface
func (w *WebSocketOutput) RegisterRoutes(router *mux.Router) {
	router.HandleFunc(w.settings.Path, w.hub.HandleConnection).Methods("GET")
	slog.Info("WebSocket route registered", "output", w.GetName(), "path", w.settings.Path)
}

// SendFormattedMetadata implements the Output interface
func (w *WebSocketOutput) SendFormattedMetadata(formattedText string) {
	// Check if value changed to avoid unnecessary broadcasts
	if !w.HasChanged(formattedText) {
		return
	}

	// Create a basic message when we don't have full metadata
	msg := &utils.UniversalMetadata{
		Type:              "metadata_update",
		FormattedMetadata: formattedText,
		Title:             formattedText,
		UpdatedAt:         time.Now(),
	}

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

	// Use the utility function to convert metadata with type
	msg := utils.ConvertMetadataWithType(formattedText, metadata, "metadata_update", inputName, inputType)

	// Store current metadata
	w.storeCurrentMetadata(msg)
	w.broadcastMessage(*msg)
}

// storeCurrentMetadata stores the current metadata for new connections
func (w *WebSocketOutput) storeCurrentMetadata(metadata *utils.UniversalMetadata) {
	w.metadataMu.Lock()
	defer w.metadataMu.Unlock()
	w.currentMetadata = metadata
}

// broadcastMessage sends a message to all connected WebSocket clients
func (w *WebSocketOutput) broadcastMessage(msg utils.UniversalMetadata) {
	payload := w.preparePayload(msg)
	w.hub.Broadcast(payload)
}

// preparePayload prepares the payload to send
func (w *WebSocketOutput) preparePayload(msg utils.UniversalMetadata) interface{} {
	if w.payloadMapper != nil {
		// Transform using payload mapper - need to convert to template data first
		payload := w.payloadMapper.MapPayload(msg.ToTemplateData())
		if payload != nil {
			return payload
		}
		// If mapping failed, fall back to original message
		slog.Debug("PayloadMapper returned nil, using original message", "output", w.GetName())
	}
	return msg
}

// String returns a string representation of the output
func (w *WebSocketOutput) String() string {
	connectedClients := w.hub.ClientCount()
	return fmt.Sprintf("WebSocketOutput{name: %s, path: %s, connections: %d, delay: %ds, hasMapping: %t}",
		w.GetName(), w.settings.Path, connectedClients, w.GetDelay(), w.payloadMapper != nil)
}
