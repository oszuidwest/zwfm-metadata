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

// NewPostOutput creates a new POST output
func NewPostOutput(name string, settings config.PostOutputConfig) *PostOutput {
	var mapper *utils.PayloadMapper
	if settings.PayloadMapping != nil {
		mapper = utils.NewPayloadMapper(settings.PayloadMapping)
	}

	output := &PostOutput{
		OutputBase:    core.NewOutputBase(name),
		settings:      settings,
		httpClient:    &http.Client{Timeout: 10 * time.Second},
		payloadMapper: mapper,
	}
	output.SetDelay(settings.Delay)
	return output
}

// SendFormattedMetadata implements the Output interface (fallback for non-enhanced usage)
func (p *PostOutput) SendFormattedMetadata(formattedText string) {
	// For POST output, we need full metadata, so create minimal payload
	minimalMetadata := &core.Metadata{
		Title:     formattedText, // Use formatted text as title fallback
		UpdatedAt: time.Now(),
	}
	payload := utils.ConvertMetadata(formattedText, minimalMetadata, "", "")

	p.sendPayload(*payload)
}

// SendEnhancedMetadata implements the EnhancedOutput interface
func (p *PostOutput) SendEnhancedMetadata(formattedText string, metadata *core.Metadata, inputName, inputType string) {
	// Check if value changed to avoid unnecessary HTTP requests
	if !p.HasChanged(formattedText) {
		return
	}

	// Build complete payload with all metadata fields
	payload := utils.ConvertMetadata(formattedText, metadata, inputName, inputType)

	p.sendPayload(*payload)
}

// sendPayload sends the complete payload to the configured URL
func (p *PostOutput) sendPayload(payload utils.UniversalMetadata) {
	var payloadToSend interface{}

	// If custom payload mapping is defined, use it
	if p.payloadMapper != nil {
		// Convert to template data for mapping
		payloadWithType := payload
		payloadWithType.Type = "post" // POST outputs don't have a type field by default
		payloadToSend = p.payloadMapper.MapPayload(payloadWithType.ToTemplateData())
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
	defer resp.Body.Close() //nolint:errcheck

	// Check response status
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Read response body for error details
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status code: %d - %s", resp.StatusCode, string(bodyBytes))
	}

	slog.Debug("Successfully sent POST", "url", p.settings.URL, "status", resp.StatusCode)

	return nil
}
