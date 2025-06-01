package outputs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
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
		utils.LogError("Failed to send POST request from output %s: %v", p.GetName(), err)
	}
}

// mapPayload maps the internal payload to custom structure based on configuration
func (p *PostOutput) mapPayload(payload PostPayload) map[string]interface{} {
	result := make(map[string]interface{})
	
	// Process the mapping configuration
	for key, value := range p.settings.PayloadMapping {
		switch v := value.(type) {
		case string:
			// Handle template strings with placeholders
			if strings.Contains(v, "${") {
				result[key] = p.replacePlaceholders(v, payload)
			} else {
				// Direct field mapping
				result[key] = p.getFieldValue(v, payload)
			}
		case map[string]interface{}:
			// Handle nested objects
			nestedResult := make(map[string]interface{})
			for nestedKey, nestedValue := range v {
				if nestedStr, ok := nestedValue.(string); ok {
					if strings.Contains(nestedStr, "${") {
						nestedResult[nestedKey] = p.replacePlaceholders(nestedStr, payload)
					} else {
						nestedResult[nestedKey] = p.getFieldValue(nestedStr, payload)
					}
				} else {
					nestedResult[nestedKey] = nestedValue
				}
			}
			result[key] = nestedResult
		default:
			// Static values
			result[key] = value
		}
	}
	
	return result
}

// replacePlaceholders replaces ${field} placeholders in template strings
func (p *PostOutput) replacePlaceholders(template string, payload PostPayload) string {
	result := template
	
	// Replace all supported placeholders
	result = strings.ReplaceAll(result, "${formatted_metadata}", payload.FormattedMetadata)
	result = strings.ReplaceAll(result, "${songID}", payload.SongID)
	result = strings.ReplaceAll(result, "${title}", payload.Title)
	result = strings.ReplaceAll(result, "${artist}", payload.Artist)
	result = strings.ReplaceAll(result, "${duration}", payload.Duration)
	result = strings.ReplaceAll(result, "${updated_at}", payload.UpdatedAt.Format(time.RFC3339))
	
	if payload.ExpiresAt != nil {
		result = strings.ReplaceAll(result, "${expires_at}", payload.ExpiresAt.Format(time.RFC3339))
	} else {
		result = strings.ReplaceAll(result, "${expires_at}", "")
	}
	
	return result
}

// getFieldValue gets a field value from the payload by name
func (p *PostOutput) getFieldValue(fieldName string, payload PostPayload) interface{} {
	switch fieldName {
	case "formatted_metadata":
		return payload.FormattedMetadata
	case "songID":
		return payload.SongID
	case "title":
		return payload.Title
	case "artist":
		return payload.Artist
	case "duration":
		return payload.Duration
	case "updated_at":
		return payload.UpdatedAt
	case "expires_at":
		return payload.ExpiresAt
	default:
		return nil
	}
}

// sendHTTPRequest sends the payload to the configured URL with bearer token
func (p *PostOutput) sendHTTPRequest(payload interface{}) error {
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
