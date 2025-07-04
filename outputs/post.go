package outputs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
	"zwfm-metadata/config"
	"zwfm-metadata/core"
	"zwfm-metadata/utils"
)

// PostOutput handles sending complete metadata via HTTP POST requests with bearer token
type PostOutput struct {
	*core.OutputBase
	core.PassiveComponent
	settings      config.PostOutputConfig
	httpClient    *http.Client
	payloadMapper *utils.PayloadMapper
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

// NewPostOutput creates a new POST output
func NewPostOutput(name string, settings config.PostOutputConfig) *PostOutput {
	var mapper *utils.PayloadMapper
	if settings.PayloadMapping != nil {
		// TODO: Remove OmitEmpty when padenc-api properly handles empty fields
		mapper = utils.NewPayloadMapperWithOmitEmpty(settings.PayloadMapping, settings.OmitEmpty)
	}

	return &PostOutput{
		OutputBase:    core.NewOutputBase(name),
		settings:      settings,
		httpClient:    &http.Client{Timeout: 10 * time.Second},
		payloadMapper: mapper,
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
	if p.payloadMapper != nil {
		// Convert to MetadataPayload for mapping
		metaPayload := utils.MetadataPayload{
			Type:              "post", // POST outputs don't have a type field by default
			FormattedMetadata: payload.FormattedMetadata,
			SongID:            payload.SongID,
			Title:             payload.Title,
			Artist:            payload.Artist,
			Duration:          payload.Duration,
			UpdatedAt:         payload.UpdatedAt,
			ExpiresAt:         payload.ExpiresAt,
		}
		payloadToSend = p.payloadMapper.MapPayload(metaPayload.ToTemplateData())
	} else {
		// Use default payload structure
		payloadToSend = payload
	}

	if err := p.sendHTTPRequest(payloadToSend); err != nil {
		slog.Error("Failed to send POST request", "output", p.GetName(), "error", err)
	}
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
	req.Header.Set("User-Agent", utils.UserAgent())

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
