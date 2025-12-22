package outputs

import (
	"encoding/json"
	"fmt"
	"html"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
	"zwfm-metadata/config"
	"zwfm-metadata/core"
	"zwfm-metadata/utils"
)

// HTTPOutput serves metadata via configurable HTTP GET endpoints.
type HTTPOutput struct {
	*core.OutputBase
	core.PassiveComponent
	settings        config.HTTPOutputConfig
	currentMetadata *utils.UniversalMetadata
	metadataMu      sync.RWMutex

	// Pre-compiled templates for performance
	endpointMappers map[string]*utils.PayloadMapper // path -> pre-compiled mapper
}

// NewHTTPOutput creates an HTTPOutput with the given name and settings.
func NewHTTPOutput(name string, settings config.HTTPOutputConfig) *HTTPOutput {
	output := &HTTPOutput{
		OutputBase:      core.NewOutputBase(name),
		settings:        settings,
		endpointMappers: make(map[string]*utils.PayloadMapper),
	}

	for _, endpoint := range settings.Endpoints {
		if endpoint.PayloadMapping != nil {
			output.endpointMappers[endpoint.Path] = utils.NewPayloadMapper(endpoint.PayloadMapping)
		}
	}

	output.SetDelay(settings.Delay)
	return output
}

// RegisterRoutes registers HTTP GET endpoints on the server mux.
func (h *HTTPOutput) RegisterRoutes(mux *http.ServeMux) {
	for _, endpoint := range h.settings.Endpoints {
		mux.HandleFunc("GET "+endpoint.Path, func(w http.ResponseWriter, req *http.Request) {
			h.handleEndpoint(w, req, endpoint)
		})

		slog.Info("HTTP endpoint registered", "output", h.GetName(), "path", endpoint.Path, "type", endpoint.ResponseType)
	}
}

// SendFormattedMetadata stores metadata for serving via HTTP endpoints.
func (h *HTTPOutput) SendFormattedMetadata(formattedText string) {
	if !h.HasChanged(formattedText) {
		return
	}

	minimalMetadata := &core.Metadata{
		Title:     formattedText, // Use formatted text as title fallback
		UpdatedAt: time.Now(),
	}

	httpMetadata := utils.ConvertMetadata(formattedText, minimalMetadata, "", "")
	h.storeCurrentMetadata(httpMetadata)
}

// SendEnhancedMetadata stores full metadata for serving via HTTP endpoints.
func (h *HTTPOutput) SendEnhancedMetadata(formattedText string, metadata *core.Metadata, inputName, inputType string) {
	if !h.HasChanged(formattedText) {
		return
	}

	httpMetadata := utils.ConvertMetadata(formattedText, metadata, inputName, inputType)
	h.storeCurrentMetadata(httpMetadata)
}

func (h *HTTPOutput) handleEndpoint(w http.ResponseWriter, _ *http.Request, endpoint config.HTTPEndpoint) {
	metadata := h.getCurrentMetadata()
	if metadata == nil {
		http.Error(w, "No metadata available", http.StatusNoContent)
		return
	}

	responseData, contentType, err := h.generateResponse(metadata, endpoint)
	if err != nil {
		slog.Error("Failed to generate HTTP response", "output", h.GetName(), "path", endpoint.Path, "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if _, err := w.Write(responseData); err != nil {
		slog.Error("Failed to write HTTP response", "output", h.GetName(), "path", endpoint.Path, "error", err)
	}

	slog.Debug("Served HTTP response", "output", h.GetName(), "path", endpoint.Path, "content_type", contentType)
}

func (h *HTTPOutput) generateResponse(metadata *utils.UniversalMetadata, endpoint config.HTTPEndpoint) ([]byte, string, error) {
	if endpoint.PayloadMapping != nil {
		return h.generateCustomResponse(metadata, endpoint)
	}
	return h.generateStandardResponse(metadata, endpoint.ResponseType)
}

func (h *HTTPOutput) generateCustomResponse(metadata *utils.UniversalMetadata, endpoint config.HTTPEndpoint) ([]byte, string, error) {
	mapper := h.endpointMappers[endpoint.Path]
	if mapper == nil {
		mapper = utils.NewPayloadMapper(endpoint.PayloadMapping)
	}

	result := mapper.MapPayload(metadata.ToTemplateData())

	if len(result) == 1 {
		for _, value := range result {
			if str, ok := value.(string); ok {
				return h.encodeResponse(str, endpoint.ResponseType)
			}
		}
	}
	return h.encodeResponse(result, endpoint.ResponseType)
}

func (h *HTTPOutput) generateStandardResponse(metadata *utils.UniversalMetadata, responseType string) ([]byte, string, error) {
	switch strings.ToLower(responseType) {
	case "xml":
		return h.encodeResponse(h.buildXMLString(metadata), responseType)
	case "plaintext", "text":
		return h.encodeResponse(metadata.FormattedMetadata, responseType)
	case "json", "":
		return h.encodeResponse(metadata, responseType)
	default:
		return nil, "", fmt.Errorf("unknown response type: %s", responseType)
	}
}

func (h *HTTPOutput) encodeResponse(data any, responseType string) ([]byte, string, error) {
	switch strings.ToLower(responseType) {
	case "xml":
		if str, ok := data.(string); ok {
			return []byte(str), "application/xml", nil
		}
		fallthrough // Complex data falls back to JSON
	case "plaintext", "text":
		if str, ok := data.(string); ok {
			return []byte(str), "text/plain", nil
		}
		fallthrough // Non-string data falls back to JSON
	case "json", "":
		encoded, err := json.Marshal(data)
		return encoded, "application/json", err
	default:
		encoded, err := json.Marshal(data)
		return encoded, "application/json", err
	}
}

func (h *HTTPOutput) buildXMLString(metadata *utils.UniversalMetadata) string {
	expiresAt := ""
	if metadata.ExpiresAt != nil {
		expiresAt = metadata.ExpiresAt.Format(time.RFC3339)
	}

	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<metadata>
    <formatted_metadata>%s</formatted_metadata>
    <songID>%s</songID>
    <title>%s</title>
    <artist>%s</artist>
    <duration>%s</duration>
    <updated_at>%s</updated_at>
    <expires_at>%s</expires_at>
</metadata>`,
		html.EscapeString(metadata.FormattedMetadata),
		html.EscapeString(metadata.SongID),
		html.EscapeString(metadata.Title),
		html.EscapeString(metadata.Artist),
		html.EscapeString(metadata.Duration),
		metadata.UpdatedAt.Format(time.RFC3339),
		expiresAt,
	)
}

func (h *HTTPOutput) storeCurrentMetadata(metadata *utils.UniversalMetadata) {
	h.metadataMu.Lock()
	defer h.metadataMu.Unlock()
	h.currentMetadata = metadata
}

func (h *HTTPOutput) getCurrentMetadata() *utils.UniversalMetadata {
	h.metadataMu.RLock()
	defer h.metadataMu.RUnlock()
	if h.currentMetadata == nil {
		return nil
	}
	metadata := *h.currentMetadata
	return &metadata
}
