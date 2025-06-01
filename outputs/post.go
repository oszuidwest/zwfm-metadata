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
	
	// TODO: Remove all PayloadMappingOmitEmpty logic when padenc-api properly handles empty fields
	// This includes the conditional checks below that skip empty values
	
	// Process the mapping configuration
	for key, value := range p.settings.PayloadMapping {
		switch v := value.(type) {
		case string:
			// Handle template strings with placeholders
			if strings.Contains(v, "${") {
				mappedValue := p.replacePlaceholders(v, payload)
				if !p.settings.PayloadMappingOmitEmpty || mappedValue != "" {
					result[key] = mappedValue
				}
			} else {
				// Direct field mapping
				fieldValue := p.getFieldValue(v, payload)
				if !p.settings.PayloadMappingOmitEmpty || (fieldValue != nil && fieldValue != "" && fieldValue != (*time.Time)(nil)) {
					result[key] = fieldValue
				}
			}
		case map[string]interface{}:
			// Handle nested objects
			nestedResult := make(map[string]interface{})
			hasValues := false
			for nestedKey, nestedValue := range v {
				if nestedStr, ok := nestedValue.(string); ok {
					if strings.Contains(nestedStr, "${") {
						mappedValue := p.replacePlaceholders(nestedStr, payload)
						if !p.settings.PayloadMappingOmitEmpty || mappedValue != "" {
							nestedResult[nestedKey] = mappedValue
							if mappedValue != "" {
								hasValues = true
							}
						}
					} else {
						fieldValue := p.getFieldValue(nestedStr, payload)
						if !p.settings.PayloadMappingOmitEmpty || (fieldValue != nil && fieldValue != "" && fieldValue != (*time.Time)(nil)) {
							nestedResult[nestedKey] = fieldValue
							if fieldValue != nil && fieldValue != "" && fieldValue != (*time.Time)(nil) {
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
			if !p.settings.PayloadMappingOmitEmpty || hasValues {
				result[key] = nestedResult
			}
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