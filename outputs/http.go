package outputs

import (
	"encoding/json"
	"fmt"
	"html"
	"log/slog"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"zwfm-metadata/config"
	"zwfm-metadata/core"
)

type httpEndpoint struct {
	path         string
	responseType string // "json", "xml", or "text"
	mapper       *PayloadMapper
}

type httpResponse struct {
	data        []byte
	contentType string
}

// HTTPOutput serves metadata via configurable HTTP GET endpoints. Responses are
// rendered once per Send and served as-is.
type HTTPOutput struct {
	*core.OutputBase
	core.PassiveComponent
	endpoints []httpEndpoint
	responses atomic.Pointer[map[string]httpResponse] // path -> rendered response
}

// NewHTTPOutput validates the endpoints and compiles their payload mappings.
func NewHTTPOutput(name string, settings config.HTTPOutputConfig) (*HTTPOutput, error) {
	output := &HTTPOutput{
		OutputBase: core.NewOutputBase(name),
		endpoints:  make([]httpEndpoint, 0, len(settings.Endpoints)),
	}

	for _, endpoint := range settings.Endpoints {
		responseType, err := normalizeResponseType(endpoint.ResponseType)
		if err != nil {
			return nil, fmt.Errorf("endpoint %q: %w", endpoint.Path, err)
		}
		mapper, err := NewPayloadMapper(endpoint.PayloadMapping)
		if err != nil {
			return nil, fmt.Errorf("endpoint %q: %w", endpoint.Path, err)
		}
		output.endpoints = append(output.endpoints, httpEndpoint{
			path:         endpoint.Path,
			responseType: responseType,
			mapper:       mapper,
		})
	}

	return output, nil
}

func normalizeResponseType(responseType string) (string, error) {
	switch strings.ToLower(responseType) {
	case "json", "":
		return "json", nil
	case "xml":
		return "xml", nil
	case "plaintext", "text":
		return "text", nil
	default:
		return "", fmt.Errorf("unknown response type: %s", responseType)
	}
}

// RegisterRoutes adds HTTP GET handlers for each configured endpoint to the mux.
func (h *HTTPOutput) RegisterRoutes(mux *http.ServeMux) {
	for _, endpoint := range h.endpoints {
		mux.HandleFunc("GET "+endpoint.path, func(w http.ResponseWriter, _ *http.Request) {
			h.serve(w, endpoint.path)
		})

		slog.Info("HTTP endpoint registered",
			"output", h.GetName(),
			"path", endpoint.path,
			"type", endpoint.responseType,
		)
	}
}

// Send renders every endpoint's response for subsequent requests.
func (h *HTTPOutput) Send(st *core.StructuredText) error {
	metadata := ConvertStructuredText(st)

	responses := make(map[string]httpResponse, len(h.endpoints))
	for _, endpoint := range h.endpoints {
		response, err := endpoint.render(metadata)
		if err != nil {
			return fmt.Errorf("render endpoint %q: %w", endpoint.path, err)
		}
		responses[endpoint.path] = response
	}

	h.responses.Store(&responses)
	return nil
}

func (h *HTTPOutput) serve(w http.ResponseWriter, path string) {
	responses := h.responses.Load()
	if responses == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	response, ok := (*responses)[path]
	if !ok {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	w.Header().Set("Content-Type", response.contentType)
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if _, err := w.Write(response.data); err != nil {
		slog.Error("Failed to write HTTP response", "output", h.GetName(), "path", path, "error", err)
	}
}

// render serves single mapped strings raw for XML and text; other mappings use JSON.
func (e httpEndpoint) render(metadata *UniversalMetadata) (httpResponse, error) {
	if e.mapper != nil {
		mapped := e.mapper.MapPayload(metadata.ToTemplateData())
		if len(mapped) == 1 {
			for _, value := range mapped {
				if str, ok := value.(string); ok {
					if e.responseType == "json" {
						return jsonResponse(str)
					}
					return rawResponse(str, e.responseType), nil
				}
			}
		}
		return jsonResponse(mapped)
	}

	switch e.responseType {
	case "xml":
		return rawResponse(buildXMLString(metadata), e.responseType), nil
	case "text":
		return rawResponse(metadata.FormattedMetadata, e.responseType), nil
	default:
		return jsonResponse(metadata)
	}
}

func rawResponse(body, responseType string) httpResponse {
	contentType := "text/plain"
	if responseType == "xml" {
		contentType = "application/xml"
	}
	return httpResponse{data: []byte(body), contentType: contentType}
}

func jsonResponse(data any) (httpResponse, error) {
	encoded, err := json.Marshal(data)
	if err != nil {
		return httpResponse{}, err
	}
	return httpResponse{data: encoded, contentType: "application/json"}, nil
}

func buildXMLString(metadata *UniversalMetadata) string {
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
