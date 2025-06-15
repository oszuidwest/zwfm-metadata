package outputs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"text/template"
	"time"
	"zwfm-metadata/config"
	"zwfm-metadata/core"
)

// PostOutput handles sending complete metadata via HTTP POST requests with bearer token
type PostOutput struct {
	*core.OutputBase
	core.PassiveComponent
	settings   config.PostOutputConfig
	httpClient *http.Client
}

// PostPayload represents the complete payload sent to the endpoint
type PostPayload struct {
	FormattedMetadata string     `json:"formatted_metadata"`
	SongID            string     `json:"songID,omitempty"`
	Title             string     `json:"title"`
	Artist            string     `json:"artist,omitempty"`
	Duration          string     `json:"duration,omitempty"`
	UpdatedAt         time.Time  `json:"updated_at"`
	ExpiresAt         *time.Time `json:"expires_at,omitempty"`
}

// bufferPool is a pool of reusable bytes.Buffer objects for template processing
var bufferPool = sync.Pool{
	New: func() interface{} {
		return new(bytes.Buffer)
	},
}

// NewPostOutput creates a new POST output
func NewPostOutput(name string, settings config.PostOutputConfig) *PostOutput {
	return &PostOutput{
		OutputBase: core.NewOutputBase(name),
		settings:   settings,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// GetDelay implements the Output interface
func (p *PostOutput) GetDelay() int {
	return p.settings.Delay
}

// SendFormattedMetadata implements the Output interface (fallback for non-enhanced usage)
func (p *PostOutput) SendFormattedMetadata(formattedText string) {
	// For POST output, we need full metadata, so create minimal payload
	payload := PostPayload{
		FormattedMetadata: formattedText,
		Title:             formattedText, // Use formatted text as title fallback
		UpdatedAt:         time.Now(),
	}

	p.sendPayload(payload)
}

// SendEnhancedMetadata implements the EnhancedOutput interface
func (p *PostOutput) SendEnhancedMetadata(formattedText string, metadata *core.Metadata) {
	// Check if value changed to avoid unnecessary HTTP requests
	if !p.HasChanged(formattedText) {
		return
	}

	// Build complete payload with all metadata fields
	payload := PostPayload{
		FormattedMetadata: formattedText,
		SongID:            metadata.SongID,
		Title:             metadata.Title,
		Artist:            metadata.Artist,
		Duration:          metadata.Duration,
		UpdatedAt:         metadata.UpdatedAt,
		ExpiresAt:         metadata.ExpiresAt,
	}

	p.sendPayload(payload)
}

// sendPayload sends the complete payload to the configured URL
func (p *PostOutput) sendPayload(payload PostPayload) {
	var payloadToSend interface{}

	// If custom payload mapping is defined, use it
	if p.settings.PayloadMapping != nil {
		mappedPayload := p.mapPayload(payload)
		payloadToSend = mappedPayload
	} else {
		// Use default payload structure
		payloadToSend = payload
	}

	if err := p.sendHTTPRequest(payloadToSend); err != nil {
		slog.Error("Failed to send POST request", "output", p.GetName(), "error", err)
	}
}

// mapPayload maps the internal payload to custom structure based on configuration
func (p *PostOutput) mapPayload(payload PostPayload) map[string]interface{} {
	result := make(map[string]interface{})

	// TODO: Remove all OmitEmpty logic when padenc-api properly handles empty fields
	// This includes the conditional checks below that skip empty values

	// Process the mapping configuration
	for key, value := range p.settings.PayloadMapping {
		switch v := value.(type) {
		case string:
			// Check if string contains template syntax {{.field}}
			if strings.Contains(v, "{{") && strings.Contains(v, "}}") {
				// Process as template
				processedValue := p.processTemplate(v, payload)
				if !p.settings.OmitEmpty || processedValue != "" {
					result[key] = processedValue
				}
			} else {
				// Use as static string value
				if !p.settings.OmitEmpty || v != "" {
					result[key] = v
				}
			}
		case map[string]interface{}:
			// Handle nested objects
			nestedResult := make(map[string]interface{})
			hasValues := false
			for nestedKey, nestedValue := range v {
				if nestedStr, ok := nestedValue.(string); ok {
					// Check if string contains template syntax {{.field}}
					if strings.Contains(nestedStr, "{{") && strings.Contains(nestedStr, "}}") {
						// Process as template
						processedValue := p.processTemplate(nestedStr, payload)
						if !p.settings.OmitEmpty || processedValue != "" {
							nestedResult[nestedKey] = processedValue
							if processedValue != "" {
								hasValues = true
							}
						}
					} else {
						// Use as static string value
						if !p.settings.OmitEmpty || nestedStr != "" {
							nestedResult[nestedKey] = nestedStr
							if nestedStr != "" {
								hasValues = true
							}
						}
					}
				} else {
					nestedResult[nestedKey] = nestedValue
					hasValues = true
				}
			}
			// For nested objects, only omit if omitEmpty is true and no values exist
			if !p.settings.OmitEmpty || hasValues {
				result[key] = nestedResult
			}
		default:
			// Static values
			result[key] = value
		}
	}

	return result
}

// processTemplate processes template strings with {{.field}} syntax
func (p *PostOutput) processTemplate(templateStr string, payload PostPayload) string {
	// Create a template with custom functions
	tmpl, err := template.New("payload").Funcs(template.FuncMap{
		"formatTime": func(t time.Time) string {
			return t.Format(time.RFC3339)
		},
		"formatTimePtr": func(t *time.Time) string {
			if t != nil {
				return t.Format(time.RFC3339)
			}
			return ""
		},
	}).Parse(templateStr)

	if err != nil {
		slog.Error("Failed to parse template", "error", err)
		return templateStr
	}

	// Create a data structure that includes formatted times
	data := struct {
		FormattedMetadata string
		SongID            string
		Title             string
		Artist            string
		Duration          string
		UpdatedAt         string
		ExpiresAt         string
	}{
		FormattedMetadata: payload.FormattedMetadata,
		SongID:            payload.SongID,
		Title:             payload.Title,
		Artist:            payload.Artist,
		Duration:          payload.Duration,
		UpdatedAt:         payload.UpdatedAt.Format(time.RFC3339),
	}

	if payload.ExpiresAt != nil {
		data.ExpiresAt = payload.ExpiresAt.Format(time.RFC3339)
	}

	// Get buffer from pool
	buf := bufferPool.Get().(*bytes.Buffer)
	defer func() {
		buf.Reset()
		bufferPool.Put(buf)
	}()

	if err := tmpl.Execute(buf, data); err != nil {
		slog.Error("Failed to execute template", "error", err)
		return templateStr
	}

	return buf.String()
}

// sendHTTPRequest sends the payload to the configured URL with bearer token
func (p *PostOutput) sendHTTPRequest(payload interface{}) error {
	// Marshal payload to JSON
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Log the request body for debugging (before any potential failures)
	slog.Debug("Sending POST request", "url", p.settings.URL, "payload", string(jsonData))

	// Create HTTP request
	req, err := http.NewRequest("POST", p.settings.URL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "ZWFM-Metadata/1.0")

	// Add bearer token authentication
	if p.settings.BearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+p.settings.BearerToken)
	}

	// Send request
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			slog.Debug("Failed to close response body", "error", err)
		}
	}()

	// Check response status
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Read response body for error details
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status code: %d - %s", resp.StatusCode, string(bodyBytes))
	}

	slog.Debug("Successfully sent POST", "url", p.settings.URL, "status", resp.StatusCode)

	return nil
}
