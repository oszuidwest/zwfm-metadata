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

	"github.com/gorilla/mux"
	"gopkg.in/yaml.v3"
)

// HTTPOutput handles serving metadata via GET endpoints
type HTTPOutput struct {
	*core.OutputBase
	core.PassiveComponent
	settings        config.HTTPOutputConfig
	currentMetadata *utils.UniversalMetadata
	metadataMu      sync.RWMutex

	// Pre-compiled templates for performance
	endpointMappers map[string]*utils.PayloadMapper // path -> pre-compiled mapper
}

// NewHTTPOutput creates a new HTTP output
func NewHTTPOutput(name string, settings config.HTTPOutputConfig) *HTTPOutput {
	output := &HTTPOutput{
		OutputBase:      core.NewOutputBase(name),
		settings:        settings,
		endpointMappers: make(map[string]*utils.PayloadMapper),
	}

	// Pre-compile templates for endpoints that have payloadMapping
	for _, endpoint := range settings.Endpoints {
		if endpoint.PayloadMapping != nil {
			output.endpointMappers[endpoint.Path] = utils.NewPayloadMapper(endpoint.PayloadMapping)
		}
	}

	output.SetDelay(settings.Delay)
	return output
}

// RegisterRoutes implements the RouteRegistrar interface
func (h *HTTPOutput) RegisterRoutes(router *mux.Router) {
	for _, endpoint := range h.settings.Endpoints {
		// Capture endpoint in closure to avoid loop variable issues
		endpoint := endpoint
		router.HandleFunc(endpoint.Path, func(w http.ResponseWriter, req *http.Request) {
			h.handleEndpoint(w, req, endpoint)
		}).Methods("GET")

		slog.Info("HTTP endpoint registered", "output", h.GetName(), "path", endpoint.Path, "type", endpoint.ResponseType)
	}
}

// SendFormattedMetadata implements the Output interface
func (h *HTTPOutput) SendFormattedMetadata(formattedText string) {
	// Check if value changed to avoid unnecessary processing
	if !h.HasChanged(formattedText) {
		return
	}

	// Create minimal metadata for fallback case
	minimalMetadata := &core.Metadata{
		Title:     formattedText, // Use formatted text as title fallback
		UpdatedAt: time.Now(),
	}

	httpMetadata := utils.ConvertMetadata(formattedText, minimalMetadata, "", "")
	h.storeCurrentMetadata(httpMetadata)
}

// SendEnhancedMetadata implements the EnhancedOutput interface
func (h *HTTPOutput) SendEnhancedMetadata(formattedText string, metadata *core.Metadata, inputName, inputType string) {
	// Check if value changed to avoid unnecessary processing
	if !h.HasChanged(formattedText) {
		return
	}

	httpMetadata := utils.ConvertMetadata(formattedText, metadata, inputName, inputType)
	h.storeCurrentMetadata(httpMetadata)
}

// handleEndpoint handles individual endpoint requests
func (h *HTTPOutput) handleEndpoint(w http.ResponseWriter, _ *http.Request, endpoint config.HTTPEndpoint) {
	// Get current metadata
	metadata := h.getCurrentMetadata()
	if metadata == nil {
		http.Error(w, "No metadata available", http.StatusNoContent)
		return
	}

	// Generate response using pre-compiled templates
	responseData, contentType, err := h.generateResponse(metadata, endpoint)
	if err != nil {
		slog.Error("Failed to generate HTTP response", "output", h.GetName(), "path", endpoint.Path, "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Set headers (let reverse proxy handle caching)
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Write response
	if _, err := w.Write(responseData); err != nil {
		slog.Error("Failed to write HTTP response", "output", h.GetName(), "path", endpoint.Path, "error", err)
	}

	slog.Debug("Served HTTP response", "output", h.GetName(), "path", endpoint.Path, "content_type", contentType)
}

// generateResponse creates the response data based on endpoint configuration
func (h *HTTPOutput) generateResponse(metadata *utils.UniversalMetadata, endpoint config.HTTPEndpoint) ([]byte, string, error) {
	// If payloadMapping is defined, use it (always takes priority)
	if endpoint.PayloadMapping != nil {
		return h.generateCustomResponse(metadata, endpoint)
	}

	// Use standard response types
	return h.generateStandardResponse(metadata, endpoint.ResponseType)
}

// generateCustomResponse generates response using payloadMapping
func (h *HTTPOutput) generateCustomResponse(metadata *utils.UniversalMetadata, endpoint config.HTTPEndpoint) ([]byte, string, error) {
	// Use pre-compiled mapper (MUCH faster!)
	mapper := h.endpointMappers[endpoint.Path]
	if mapper == nil {
		// Fallback if mapper not found (shouldn't happen)
		mapper = utils.NewPayloadMapper(endpoint.PayloadMapping)
	}

	// Apply payload mapping with pre-compiled templates
	result := mapper.MapPayload(metadata.ToTemplateData())

	// Handle special cases for single-value responses
	if len(result) == 1 {
		for _, value := range result {
			if str, ok := value.(string); ok {
				return h.encodeResponse(str, endpoint.ResponseType)
			}
		}
	}

	// Handle complex structures
	return h.encodeResponse(result, endpoint.ResponseType)
}

// generateStandardResponse generates response using standard formats
func (h *HTTPOutput) generateStandardResponse(metadata *utils.UniversalMetadata, responseType string) ([]byte, string, error) {
	switch strings.ToLower(responseType) {
	case "xml":
		return h.encodeResponse(h.buildXMLString(metadata), responseType)
	case "yaml":
		return h.encodeResponse(metadata, responseType)
	case "plaintext", "text":
		return h.encodeResponse(metadata.FormattedMetadata, responseType)
	case "json", "":
		return h.encodeResponse(metadata, responseType)
	default:
		return nil, "", fmt.Errorf("unknown response type: %s", responseType)
	}
}

// encodeResponse encodes data based on response type
func (h *HTTPOutput) encodeResponse(data interface{}, responseType string) ([]byte, string, error) {
	switch strings.ToLower(responseType) {
	case "xml":
		if str, ok := data.(string); ok {
			return []byte(str), "application/xml", nil
		}
		fallthrough // Complex data falls back to JSON
	case "yaml":
		encoded, err := yaml.Marshal(data)
		return encoded, "application/x-yaml", err
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

// buildXMLString creates XML string from metadata
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

// storeCurrentMetadata stores the current metadata for endpoint requests
func (h *HTTPOutput) storeCurrentMetadata(metadata *utils.UniversalMetadata) {
	h.metadataMu.Lock()
	defer h.metadataMu.Unlock()
	h.currentMetadata = metadata
}

// getCurrentMetadata retrieves the current metadata for endpoint requests
func (h *HTTPOutput) getCurrentMetadata() *utils.UniversalMetadata {
	h.metadataMu.RLock()
	defer h.metadataMu.RUnlock()
	if h.currentMetadata == nil {
		return nil
	}
	// Return a copy to avoid race conditions
	metadata := *h.currentMetadata
	return &metadata
}
