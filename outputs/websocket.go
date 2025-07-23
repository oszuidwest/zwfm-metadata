package outputs

import (
	"log/slog"
	"sync"
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

	// Create a message based on our configuration
	msg := w.createMetadataMessage(formattedText, nil, "", "")

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

	// Create a message with full metadata
	msg := w.createMetadataMessage(formattedText, metadata, inputName, inputType)

	// Store current metadata
	w.storeCurrentMetadata(msg)
	w.broadcastMessage(*msg)
}

// createMetadataMessage creates a metadata message based on available data
func (w *WebSocketOutput) createMetadataMessage(formattedText string, metadata *core.Metadata, inputName, inputType string) *utils.UniversalMetadata {
	msg := &utils.UniversalMetadata{
		FormattedText: formattedText,
		InputName:     inputName,
		InputType:     inputType,
		Timestamp:     utils.GetFormattedTimestamp(),
	}

	// Include detailed metadata if available
	if metadata != nil {
		msg.Artist = metadata.Artist
		msg.Title = metadata.Title
		msg.SongID = metadata.SongID
		msg.ExpiresAt = metadata.ExpiresAt
		msg.Duration = metadata.Duration
		msg.StartTime = metadata.StartTime
		msg.EndTime = metadata.EndTime
	}

	return msg
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
		// Transform using payload mapper
		payload, err := w.payloadMapper.MapPayload(msg)
		if err != nil {
			slog.Error("Failed to map payload for WebSocket",
				"output", w.GetName(),
				"error", err)
			return msg
		}
		return payload
	}
	return msg
}

// String returns a string representation of the output
func (w *WebSocketOutput) String() string {
	connectedClients := w.hub.ClientCount()
	return utils.FormatComponent("WebSocketOutput", w.GetName(), map[string]interface{}{
		"path":        w.settings.Path,
		"hasChanged":  w.GetChanged(),
		"connections": connectedClients,
		"delay":       w.GetDelay(),
		"hasMapping":  w.payloadMapper != nil,
	})
}