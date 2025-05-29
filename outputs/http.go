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

// HTTPOutput handles sending metadata via HTTP POST requests
type HTTPOutput struct {
	*core.BaseOutput
	core.WaitForShutdown
	settings   config.HTTPOutputSettings
	httpClient *http.Client
}

// NewHTTPOutput creates a new HTTP POST output
func NewHTTPOutput(name string, settings config.HTTPOutputSettings) *HTTPOutput {
	return &HTTPOutput{
		BaseOutput: core.NewBaseOutput(name),
		settings:   settings,
		httpClient: utils.CreateHTTPClient(10 * time.Second),
	}
}

// GetDelay implements the Output interface
func (h *HTTPOutput) GetDelay() int {
	return h.settings.Delay
}

// ProcessFormattedMetadata implements the Output interface
func (h *HTTPOutput) ProcessFormattedMetadata(formattedText string) {
	h.processMetadata(formattedText, nil)
}

// ProcessFormattedMetadataWithDetails implements the EnhancedOutput interface
func (h *HTTPOutput) ProcessFormattedMetadataWithDetails(formattedText string, metadata *core.Metadata) {
	h.processMetadata(formattedText, metadata)
}

// processMetadata handles the actual HTTP POST request
func (h *HTTPOutput) processMetadata(formattedText string, metadata *core.Metadata) {
	// Check if value changed to avoid unnecessary HTTP requests
	if !h.HasChanged(formattedText) {
		return
	}

	// Build simple payload
	payload := map[string]interface{}{
		"formatted_metadata": formattedText,
	}

	// Add metadata fields if available
	if metadata != nil {
		payload["title"] = metadata.Title
		if metadata.Artist != "" {
			payload["artist"] = metadata.Artist
		}
		if metadata.ExpiresAt != nil {
			payload["expires_at"] = metadata.ExpiresAt.Format(time.RFC3339)
		}
	}

	// Send HTTP POST request
	if err := h.sendHTTPRequest(payload); err != nil {
		utils.LogError("Failed to send HTTP request from output %s: %v", h.GetName(), err)
	}
}

// sendHTTPRequest sends the payload to the configured URL
func (h *HTTPOutput) sendHTTPRequest(payload map[string]interface{}) error {
	// Marshal payload to JSON
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequest("POST", h.settings.URL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")

	// Add bearer token if configured
	if h.settings.BearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+h.settings.BearerToken)
	}

	// Send request
	resp, err := h.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer utils.CloseBody(resp.Body)

	// Check response status
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	utils.LogDebug("Successfully sent HTTP POST to %s (%d): %s", h.settings.URL, resp.StatusCode, string(jsonData))

	return nil
}
