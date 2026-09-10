package outputs

import (
	"log/slog"
	"net/http"
	"sync/atomic"

	"zwfm-metadata/config"
	"zwfm-metadata/core"
	"zwfm-metadata/utils"
)

// WebSocketOutput broadcasts metadata to connected WebSocket clients in real-time.
type WebSocketOutput struct {
	*core.OutputBase
	core.PassiveComponent
	settings      config.WebSocketOutputConfig
	hub           *utils.WebSocketHub
	current       atomic.Pointer[UniversalMetadata] // replayed to newly connected clients
	payloadMapper *PayloadMapper
}

// NewWebSocketOutput creates a WebSocketOutput with the given name and settings.
func NewWebSocketOutput(name string, settings config.WebSocketOutputConfig) (*WebSocketOutput, error) {
	mapper, err := NewPayloadMapper(settings.PayloadMapping)
	if err != nil {
		return nil, err
	}

	output := &WebSocketOutput{
		OutputBase:    core.NewOutputBase(name),
		settings:      settings,
		hub:           utils.NewWebSocketHub(name),
		payloadMapper: mapper,
	}

	output.hub.SetOnConnect(func() any {
		if current := output.current.Load(); current != nil {
			return output.payloadMapper.Apply(current)
		}
		return nil
	})

	return output, nil
}

// RegisterRoutes registers the WebSocket endpoint on the server mux.
func (w *WebSocketOutput) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET "+w.settings.Path, w.hub.HandleConnection)
	slog.Info("WebSocket route registered", "output", w.GetName(), "path", w.settings.Path)
}

// Send broadcasts metadata to all connected WebSocket clients.
func (w *WebSocketOutput) Send(st *core.StructuredText) {
	msg := ConvertStructuredText(st)
	msg.Type = "metadata_update"

	w.current.Store(msg)
	w.hub.Broadcast(w.payloadMapper.Apply(msg))
}
