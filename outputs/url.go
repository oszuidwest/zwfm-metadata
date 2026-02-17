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

	"zwfm-metadata/config"
	"zwfm-metadata/core"
	"zwfm-metadata/utils"
)

// URLOutput sends metadata via configurable HTTP GET or POST requests.
type URLOutput struct {
	*core.OutputBase
	core.PassiveComponent
	settings      config.URLOutputConfig
	payloadMapper *utils.PayloadMapper
	urlTemplate   *template.Template
}

// NewURLOutput creates a URLOutput with the given name and settings.
func NewURLOutput(name string, settings config.URLOutputConfig) *URLOutput {
	var mapper *utils.PayloadMapper
	if settings.PayloadMapping != nil {
		mapper = utils.NewPayloadMapper(settings.PayloadMapping)
	}

	if settings.Method == "" {
		slog.Error("Method is required for URL output", "output", name)
		return nil
	}

	settings.Method = strings.ToUpper(settings.Method)
	if settings.Method != "GET" && settings.Method != "POST" {
		slog.Error("Invalid method for URL output", "output", name, "method", settings.Method, "valid_methods", "GET, POST")
		return nil
	}

	parsedURL, err := url.Parse(settings.URL)
	if err != nil {
		slog.Error("Invalid URL", "output", name, "url", settings.URL, "error", err)
		return nil
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		slog.Error("URL must use http or https scheme", "output", name, "url", settings.URL, "scheme", parsedURL.Scheme) //nolint:gosec // Logging config value for diagnostics
		return nil
	}

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
		payloadMapper: mapper,
		urlTemplate:   tmpl,
	}
	output.SetDelay(settings.Delay)
	return output
}

// Send sends metadata via the configured HTTP method.
func (u *URLOutput) Send(st *core.StructuredText) {
	payload := utils.ConvertStructuredText(st)
	u.sendRequest(payload)
}

func (u *URLOutput) sendRequest(payload *utils.UniversalMetadata) {
	if u.settings.Method == "GET" {
		u.sendGETRequest(payload)
	} else {
		u.sendPOSTRequest(payload)
	}
}

func urlEncodeTemplateData(data map[string]any) map[string]any {
	encoded := make(map[string]any)
	for key, value := range data {
		switch v := value.(type) {
		case string:
			encoded[key] = url.QueryEscape(v)
		case map[string]any:
			encoded[key] = urlEncodeTemplateData(v)
		default:
			encoded[key] = v
		}
	}
	return encoded
}

func (u *URLOutput) sendGETRequest(payload *utils.UniversalMetadata) {
	var requestURL string

	if u.urlTemplate != nil {
		templateData := payload.ToTemplateData()
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

	parsedURL, err := url.Parse(requestURL)
	if err != nil {
		slog.Error("Failed to parse URL", "output", u.GetName(), "url", requestURL, "error", err)
		return
	}

	finalURL := parsedURL.String()

	slog.Debug("Sending GET request", "output", u.GetName(), "url", finalURL) //nolint:gosec // Logging URL for diagnostics

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, finalURL, http.NoBody)
	if err != nil {
		slog.Error("Failed to create GET request", "output", u.GetName(), "error", err)
		return
	}

	if u.settings.BearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+u.settings.BearerToken)
	}

	resp, err := utils.Do(req)
	if err != nil {
		slog.Error("Failed to send GET request", "output", u.GetName(), "error", err)
		return
	}
	defer resp.Body.Close() //nolint:errcheck // Best-effort cleanup

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		slog.Error("GET request failed", "output", u.GetName(), "status", resp.StatusCode, "response", string(bodyBytes)) //nolint:gosec // Logging response for diagnostics
		return
	}

	slog.Debug("Successfully sent GET", "output", u.GetName(), "url", finalURL, "status", resp.StatusCode) //nolint:gosec // Logging URL for diagnostics
}

func (u *URLOutput) sendPOSTRequest(payload *utils.UniversalMetadata) {
	var payloadToSend any

	if u.payloadMapper != nil {
		payload.Type = "url"
		payloadToSend = u.payloadMapper.MapPayload(payload.ToTemplateData())
	} else {
		payloadToSend = payload
	}

	jsonData, err := json.Marshal(payloadToSend)
	if err != nil {
		slog.Error("Failed to marshal payload", "output", u.GetName(), "error", err)
		return
	}

	slog.Debug("Sending POST request", "output", u.GetName(), "url", u.settings.URL, "payload", string(jsonData))

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, u.settings.URL, bytes.NewBuffer(jsonData))
	if err != nil {
		slog.Error("Failed to create POST request", "output", u.GetName(), "error", err)
		return
	}

	req.Header.Set("Content-Type", "application/json")
	if u.settings.BearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+u.settings.BearerToken)
	}

	resp, err := utils.Do(req)
	if err != nil {
		slog.Error("Failed to send POST request", "output", u.GetName(), "error", err)
		return
	}
	defer resp.Body.Close() //nolint:errcheck // Best-effort cleanup

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		slog.Error("POST request failed", "output", u.GetName(), "status", resp.StatusCode, "response", string(bodyBytes)) //nolint:gosec // Logging response for diagnostics
		return
	}

	slog.Debug("Successfully sent POST", "output", u.GetName(), "url", u.settings.URL, "status", resp.StatusCode) //nolint:gosec // Logging URL for diagnostics
}
