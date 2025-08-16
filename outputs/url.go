package outputs

import (
	"bytes"
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

// URLOutput handles sending metadata via HTTP GET or POST requests
type URLOutput struct {
	*core.OutputBase
	core.PassiveComponent
	settings      config.URLOutputConfig
	httpClient    *http.Client
	payloadMapper *utils.PayloadMapper
	urlTemplate   *template.Template
}

// NewURLOutput creates a new URL output
func NewURLOutput(name string, settings config.URLOutputConfig) *URLOutput {
	var mapper *utils.PayloadMapper
	if settings.PayloadMapping != nil {
		mapper = utils.NewPayloadMapper(settings.PayloadMapping)
	}

	// Parse URL template if it contains template syntax
	var tmpl *template.Template
	if strings.Contains(settings.URL, "{{") {
		var err error
		tmpl, err = template.New("url").Parse(settings.URL)
		if err != nil {
			slog.Error("Failed to parse URL template", "error", err)
			return nil
		}
	}

	// Validate method is specified and valid
	if settings.Method == "" {
		slog.Error("Method is required for URL output", "output", name)
		return nil
	}

	// Validate method is GET or POST (case-insensitive)
	method := strings.ToUpper(settings.Method)
	if method != "GET" && method != "POST" {
		slog.Error("Invalid method for URL output", "output", name, "method", settings.Method, "valid_methods", "GET, POST")
		return nil
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

// SendFormattedMetadata implements the Output interface (fallback for non-enhanced usage)
func (u *URLOutput) SendFormattedMetadata(formattedText string) {
	minimalMetadata := &core.Metadata{
		Title:     formattedText,
		UpdatedAt: time.Now(),
	}
	payload := utils.ConvertMetadata(formattedText, minimalMetadata, "", "")

	u.sendRequest(*payload)
}

// SendEnhancedMetadata implements the EnhancedOutput interface
func (u *URLOutput) SendEnhancedMetadata(formattedText string, metadata *core.Metadata, inputName, inputType string) {
	// Check if value changed to avoid unnecessary HTTP requests
	if !u.HasChanged(formattedText) {
		return
	}

	// Build complete payload with all metadata fields
	payload := utils.ConvertMetadata(formattedText, metadata, inputName, inputType)

	u.sendRequest(*payload)
}

// sendRequest sends the request based on configured method
func (u *URLOutput) sendRequest(payload utils.UniversalMetadata) {
	if strings.ToUpper(u.settings.Method) == "GET" {
		u.sendGETRequest(payload)
	} else {
		u.sendPOSTRequest(payload)
	}
}

// sendGETRequest sends metadata as GET request with query parameters
func (u *URLOutput) sendGETRequest(payload utils.UniversalMetadata) {
	var requestURL string

	// If URL contains templates, process them
	if u.urlTemplate != nil {
		var urlBuffer strings.Builder
		if err := u.urlTemplate.Execute(&urlBuffer, payload.ToTemplateData()); err != nil {
			slog.Error("Failed to build URL from template", "output", u.GetName(), "error", err)
			return
		}
		requestURL = urlBuffer.String()
	} else {
		requestURL = u.settings.URL
	}

	// Parse and ensure proper URL encoding
	parsedURL, err := url.Parse(requestURL)
	if err != nil {
		slog.Error("Failed to parse URL", "output", u.GetName(), "url", requestURL, "error", err)
		return
	}

	// Re-encode query parameters to ensure proper encoding
	if parsedURL.RawQuery != "" {
		query := parsedURL.Query()
		parsedURL.RawQuery = query.Encode()
	}

	finalURL := parsedURL.String()

	slog.Debug("Sending GET request", "url", finalURL)

	// Create HTTP request
	req, err := http.NewRequest("GET", finalURL, nil)
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

	slog.Debug("Successfully sent GET", "url", finalURL, "status", resp.StatusCode)
}

// sendPOSTRequest sends metadata as POST request with JSON body
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

	slog.Debug("Sending POST request", "url", u.settings.URL, "payload", string(jsonData))

	// Create HTTP request
	req, err := http.NewRequest("POST", u.settings.URL, bytes.NewBuffer(jsonData))
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

	slog.Debug("Successfully sent POST", "url", u.settings.URL, "status", resp.StatusCode)
}
