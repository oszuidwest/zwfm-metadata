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
)

// HTTPOutput serves metadata via configurable HTTP GET endpoints.
type HTTPOutput struct {
	*core.OutputBase
	core.PassiveComponent
	settings        config.HTTPOutputConfig
	currentMetadata *UniversalMetadata
	metadataMu      sync.RWMutex

	// Pre-compiled templates for performance
	endpointMappers map[string]*PayloadMapper // path -> pre-compiled mapper
}

// NewHTTPOutput initializes an HTTP endpoint server with the given settings.
func NewHTTPOutput(name string, settings config.HTTPOutputConfig) *HTTPOutput {
	output := &HTTPOutput{
		OutputBase:      core.NewOutputBase(name),
		settings:        settings,
		endpointMappers: make(map[string]*PayloadMapper),
	}

	for _, endpoint := range settings.Endpoints {
		if endpoint.PayloadMapping != nil {
			output.endpointMappers[endpoint.Path] = NewPayloadMapper(endpoint.PayloadMapping)
		}
	}

	output.SetDelay(settings.Delay)
	return output
}

// RegisterRoutes adds HTTP GET handlers for each configured endpoint to the mux.
func (h *HTTPOutput) RegisterRoutes(mux *http.ServeMux) {
	for _, endpoint := range h.settings.Endpoints {
		mux.HandleFunc("GET "+endpoint.Path, func(w http.ResponseWriter, req *http.Request) {
			h.handleEndpoint(w, req, endpoint)
		})

		slog.Info("HTTP endpoint registered",
			"output", h.GetName(),
			"path", endpoint.Path,
			"type", endpoint.ResponseType,
		)
	}
}

// Send caches the metadata for subsequent HTTP endpoint responses.
func (h *HTTPOutput) Send(st *core.StructuredText) {
	httpMetadata := ConvertStructuredText(st)
	h.storeCurrentMetadata(httpMetadata)
}

func (h *HTTPOutput) handleEndpoint(
	w http.ResponseWriter,
	_ *http.Request,
	endpoint config.HTTPEndpoint,
) {
	metadata := h.getCurrentMetadata()
	if metadata == nil {
		http.Error(w, "No metadata available", http.StatusNoContent)
		return
	}

	responseData, contentType, err := h.generateResponse(metadata, endpoint)
	if err != nil {
		slog.Error("Failed to generate HTTP response",
			"output", h.GetName(),
			"path", endpoint.Path,
			"error", err,
		)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if _, err := w.Write(responseData); err != nil {
		slog.Error("Failed to write HTTP response",
			"output", h.GetName(),
			"path", endpoint.Path,
			"error", err,
		)
	}

	slog.Debug("Served HTTP response",
		"output", h.GetName(),
		"path", endpoint.Path,
		"content_type", contentType,
	)
}

func (h *HTTPOutput) generateResponse(
	metadata *UniversalMetadata, endpoint config.HTTPEndpoint,
) (data []byte, contentType string, err error) {
	if endpoint.PayloadMapping != nil {
		return h.generateCustomResponse(metadata, endpoint)
	}
	return h.generateStandardResponse(metadata, endpoint.ResponseType)
}

func (h *HTTPOutput) generateCustomResponse(
	metadata *UniversalMetadata, endpoint config.HTTPEndpoint,
) (data []byte, contentType string, err error) {
	result := h.endpointMappers[endpoint.Path].MapPayload(metadata.ToTemplateData())

	if len(result) == 1 {
		for _, value := range result {
			if str, ok := value.(string); ok {
				return h.encodeResponse(str, endpoint.ResponseType)
			}
		}
	}
	return h.encodeResponse(result, endpoint.ResponseType)
}

func (h *HTTPOutput) generateStandardResponse(
	metadata *UniversalMetadata, responseType string,
) (data []byte, contentType string, err error) {
	switch strings.ToLower(responseType) {
	case "xml":
		return []byte(h.buildXMLString(metadata)), "application/xml", nil
	case "plaintext", "text":
		return []byte(metadata.FormattedMetadata), "text/plain", nil
	case "json", "":
		encoded, err := json.Marshal(metadata)
		return encoded, "application/json", err
	default:
		return nil, "", fmt.Errorf("unknown response type: %s", responseType)
	}
}

// encodeResponse serves a custom-mapped value: strings are served raw for xml and
// plaintext response types; everything else is JSON.
func (h *HTTPOutput) encodeResponse(
	data any,
	responseType string,
) (encoded []byte, contentType string, err error) {
	if str, ok := data.(string); ok {
		switch strings.ToLower(responseType) {
		case "xml":
			return []byte(str), "application/xml", nil
		case "plaintext", "text":
			return []byte(str), "text/plain", nil
		}
	}

	encoded, err = json.Marshal(data)
	return encoded, "application/json", err
}

func (h *HTTPOutput) buildXMLString(metadata *UniversalMetadata) string {
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

func (h *HTTPOutput) storeCurrentMetadata(metadata *UniversalMetadata) {
	h.metadataMu.Lock()
	defer h.metadataMu.Unlock()
	h.currentMetadata = metadata
}

func (h *HTTPOutput) getCurrentMetadata() *UniversalMetadata {
	h.metadataMu.RLock()
	defer h.metadataMu.RUnlock()
	if h.currentMetadata == nil {
		return nil
	}
	metadata := *h.currentMetadata
	return &metadata
}
