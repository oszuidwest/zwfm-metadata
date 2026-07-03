package outputs

import (
	"log/slog"
	"net/http"
	"sync"

	"zwfm-metadata/config"
	"zwfm-metadata/core"
	"zwfm-metadata/utils"
)

// WebSocketOutput broadcasts metadata to connected WebSocket clients in real-time.
type WebSocketOutput struct {
	*core.OutputBase
	core.PassiveComponent
	settings        config.WebSocketOutputConfig
	hub             *utils.WebSocketHub
	currentMetadata *UniversalMetadata
	metadataMu      sync.RWMutex
	payloadMapper   *PayloadMapper
}

// NewWebSocketOutput creates a WebSocketOutput with the given name and settings.
func NewWebSocketOutput(name string, settings config.WebSocketOutputConfig) *WebSocketOutput {
	var mapper *PayloadMapper
	if settings.PayloadMapping != nil {
		mapper = NewPayloadMapper(settings.PayloadMapping)
	}

	output := &WebSocketOutput{
		OutputBase:    core.NewOutputBase(name),
		settings:      settings,
		hub:           utils.NewWebSocketHub(name),
		payloadMapper: mapper,
	}
	output.SetDelay(settings.Delay)

	output.hub.SetOnConnect(func() any {
		output.metadataMu.RLock()
		defer output.metadataMu.RUnlock()
		if output.currentMetadata != nil {
			return output.preparePayload(output.currentMetadata)
		}
		return nil
	})

	return output
}

// RegisterRoutes registers the WebSocket endpoint on the server mux.
func (w *WebSocketOutput) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET "+w.settings.Path, w.hub.HandleConnection)
	slog.Info("WebSocket route registered", "output", w.GetName(), "path", w.settings.Path)
}

// Send broadcasts metadata to all connected WebSocket clients.
func (w *WebSocketOutput) Send(st *core.StructuredText) {
	msg := ConvertStructuredTextWithType(st, "metadata_update")

	w.storeCurrentMetadata(msg)
	w.hub.Broadcast(w.preparePayload(msg))
}

func (w *WebSocketOutput) storeCurrentMetadata(metadata *UniversalMetadata) {
	w.metadataMu.Lock()
	defer w.metadataMu.Unlock()
	w.currentMetadata = metadata
}

func (w *WebSocketOutput) preparePayload(msg *UniversalMetadata) any {
	if w.payloadMapper != nil {
		payload := w.payloadMapper.MapPayload(msg.ToTemplateData())
		if payload != nil {
			return payload
		}
		slog.Debug("PayloadMapper returned nil, using original message", "output", w.GetName())
	}
	return msg
}
