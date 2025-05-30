package outputs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
	"zwfm-metadata/config"
	"zwfm-metadata/core"
	"zwfm-metadata/utils"
)

// PostOutput handles sending complete metadata via HTTP POST requests with bearer token
type PostOutput struct {
	*core.BaseOutput
	core.WaitForShutdown
	settings   config.PostOutputSettings
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

// NewPostOutput creates a new POST output
func NewPostOutput(name string, settings config.PostOutputSettings) *PostOutput {
	return &PostOutput{
		BaseOutput: core.NewBaseOutput(name),
		settings:   settings,
		httpClient: utils.CreateHTTPClient(10 * time.Second),
	}
}

// GetDelay implements the Output interface
func (p *PostOutput) GetDelay() int {
	return p.settings.Delay
}

// ProcessFormattedMetadata implements the Output interface (fallback for non-enhanced usage)
func (p *PostOutput) ProcessFormattedMetadata(formattedText string) {
	// For POST output, we need full metadata, so create minimal payload
	payload := PostPayload{
		FormattedMetadata: formattedText,
		Title:             formattedText, // Use formatted text as title fallback
		UpdatedAt:         time.Now(),
	}

	p.sendPayload(payload)
}

// ProcessEnhancedMetadata implements the EnhancedOutput interface
func (p *PostOutput) ProcessEnhancedMetadata(formattedText string, metadata *core.Metadata) {
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
	if err := p.sendHTTPRequest(payload); err != nil {
		utils.LogError("Failed to send POST request from output %s: %v", p.GetName(), err)
	}
}

// sendHTTPRequest sends the payload to the configured URL with bearer token
func (p *PostOutput) sendHTTPRequest(payload PostPayload) error {
	// Marshal payload to JSON
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

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
	defer utils.CloseBody(resp.Body)

	// Check response status
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	utils.LogDebug("Successfully sent POST to %s (%d): %s", p.settings.URL, resp.StatusCode, string(jsonData))

	return nil
}
