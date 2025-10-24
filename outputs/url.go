package outputs

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"text/template"
	"time"
	"zwfm-metadata/config"
	"zwfm-metadata/core"
	"zwfm-metadata/utils"
)

// URLOutput handles sending metadata via HTTP GET or POST requests.
type URLOutput struct {
	*core.OutputBase
	core.PassiveComponent
	settings      config.URLOutputConfig
	httpClient    *http.Client
	payloadMapper *utils.PayloadMapper
	urlTemplate   *template.Template
}

// NewURLOutput creates a new URL output.
func NewURLOutput(name string, settings config.URLOutputConfig) *URLOutput {
	var mapper *utils.PayloadMapper
	if settings.PayloadMapping != nil {
		mapper = utils.NewPayloadMapper(settings.PayloadMapping)
	}

	// Validate method is specified and valid
	if settings.Method == "" {
		slog.Error("Method is required for URL output", "output", name)
		return nil
	}

	// Normalize and validate method (case-insensitive)
	settings.Method = strings.ToUpper(settings.Method)
	if settings.Method != "GET" && settings.Method != "POST" {
		slog.Error("Invalid method for URL output", "output", name, "method", settings.Method, "valid_methods", "GET, POST")
		return nil
	}

	// Validate URL scheme
	parsedURL, err := url.Parse(settings.URL)
	if err != nil {
		slog.Error("Invalid URL", "output", name, "url", settings.URL, "error", err)
		return nil
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		slog.Error("URL must use http or https scheme", "output", name, "url", settings.URL, "scheme", parsedURL.Scheme)
		return nil
	}

	// Parse URL template if it contains template syntax
	var tmpl *template.Template
	if strings.Contains(settings.URL, "{{") {
		tmpl, err = template.New("url").Funcs(template.FuncMap{
			"upper": strings.ToUpper,
			"lower": strings.ToLower,
			"trim":  strings.TrimSpace,
		}).Parse(settings.URL)
		if err != nil {
			slog.Error("Failed to parse URL template", "output", name, "url", settings.URL, "error", err)
			return nil
		}
	}

	output := &URLOutput{
		OutputBase:    core.NewOutputBase(name),
		settings:      settings,
		httpClient:    &http.Client{Timeout: 10 * time.Second},
		payloadMapper: mapper,
		urlTemplate:   tmpl,
	}
	output.SetDelay(settings.Delay)
	return output
}

// SendFormattedMetadata implements the Output interface (fallback for non-enhanced usage).
func (u *URLOutput) SendFormattedMetadata(formattedText string) {
	// Check if value changed to avoid unnecessary HTTP requests
	if !u.HasChanged(formattedText) {
		return
	}

	minimalMetadata := &core.Metadata{
		Title:     formattedText,
		UpdatedAt: time.Now(),
	}
	payload := utils.ConvertMetadata(formattedText, minimalMetadata, "", "")

	u.sendRequest(*payload)
}

// SendEnhancedMetadata implements the EnhancedOutput interface.
func (u *URLOutput) SendEnhancedMetadata(formattedText string, metadata *core.Metadata, inputName, inputType string) {
	// Check if value changed to avoid unnecessary HTTP requests
	if !u.HasChanged(formattedText) {
		return
	}

	// Build complete payload with all metadata fields
	payload := utils.ConvertMetadata(formattedText, metadata, inputName, inputType)

	u.sendRequest(*payload)
}

// sendRequest sends the request based on configured method.
func (u *URLOutput) sendRequest(payload utils.UniversalMetadata) {
	// Method is already normalized to uppercase in constructor
	if u.settings.Method == "GET" {
		u.sendGETRequest(payload)
	} else {
		u.sendPOSTRequest(payload)
	}
}

// urlEncodeTemplateData recursively URL-encodes all string values in template data
// to ensure they are safe for use in URL query parameters.
func urlEncodeTemplateData(data map[string]interface{}) map[string]interface{} {
	encoded := make(map[string]interface{})
	for key, value := range data {
		switch v := value.(type) {
		case string:
			// Encode string values
			encoded[key] = url.QueryEscape(v)
		case map[string]interface{}:
			// Recursively encode nested maps
			encoded[key] = urlEncodeTemplateData(v)
		default:
			// Preserve non-string types
			encoded[key] = v
		}
	}
	return encoded
}

// sendGETRequest sends metadata as GET request with query parameters.
func (u *URLOutput) sendGETRequest(payload utils.UniversalMetadata) {
	var requestURL string

	// If URL contains templates, process them
	if u.urlTemplate != nil {
		templateData := payload.ToTemplateData()

		// Pre-encode template data to ensure special characters are properly escaped in URLs
		encodedData := urlEncodeTemplateData(templateData)

		var urlBuffer strings.Builder
		if err := u.urlTemplate.Execute(&urlBuffer, encodedData); err != nil {
			slog.Error("Failed to execute URL template", "output", u.GetName(), "template", u.settings.URL, "error", err)
			return
		}
		requestURL = urlBuffer.String()
	} else {
		requestURL = u.settings.URL
	}

	// Validate the constructed URL is well-formed
	parsedURL, err := url.Parse(requestURL)
	if err != nil {
		slog.Error("Failed to parse URL", "output", u.GetName(), "url", requestURL, "error", err)
		return
	}

	finalURL := parsedURL.String()

	slog.Debug("Sending GET request", "output", u.GetName(), "url", finalURL)

	// Create HTTP request with context
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", finalURL, nil)
	if err != nil {
		slog.Error("Failed to create GET request", "output", u.GetName(), "error", err)
		return
	}

	// Set headers
	req.Header.Set("User-Agent", utils.UserAgent())

	// Add bearer token authentication if configured
	if u.settings.BearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+u.settings.BearerToken)
	}

	// Send request
	resp, err := u.httpClient.Do(req)
	if err != nil {
		slog.Error("Failed to send GET request", "output", u.GetName(), "error", err)
		return
	}
	defer resp.Body.Close() //nolint:errcheck

	// Check response status
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		slog.Error("GET request failed", "output", u.GetName(), "status", resp.StatusCode, "response", string(bodyBytes))
		return
	}

	slog.Debug("Successfully sent GET", "output", u.GetName(), "url", finalURL, "status", resp.StatusCode)
}

// sendPOSTRequest sends metadata as POST request with JSON body.
func (u *URLOutput) sendPOSTRequest(payload utils.UniversalMetadata) {
	var payloadToSend interface{}

	// If custom payload mapping is defined, use it
	if u.payloadMapper != nil {
		payloadWithType := payload
		payloadWithType.Type = "url"
		payloadToSend = u.payloadMapper.MapPayload(payloadWithType.ToTemplateData())
	} else {
		payloadToSend = payload
	}

	// Marshal payload to JSON
	jsonData, err := json.Marshal(payloadToSend)
	if err != nil {
		slog.Error("Failed to marshal payload", "output", u.GetName(), "error", err)
		return
	}

	slog.Debug("Sending POST request", "output", u.GetName(), "url", u.settings.URL, "payload", string(jsonData))

	// Create HTTP request with context
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", u.settings.URL, bytes.NewBuffer(jsonData))
	if err != nil {
		slog.Error("Failed to create POST request", "output", u.GetName(), "error", err)
		return
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", utils.UserAgent())

	// Add bearer token authentication
	if u.settings.BearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+u.settings.BearerToken)
	}

	// Send request
	resp, err := u.httpClient.Do(req)
	if err != nil {
		slog.Error("Failed to send POST request", "output", u.GetName(), "error", err)
		return
	}
	defer resp.Body.Close() //nolint:errcheck

	// Check response status
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		slog.Error("POST request failed", "output", u.GetName(), "status", resp.StatusCode, "response", string(bodyBytes))
		return
	}

	slog.Debug("Successfully sent POST", "output", u.GetName(), "url", u.settings.URL, "status", resp.StatusCode)
}
