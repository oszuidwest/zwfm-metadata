package outputs

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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
	payloadMapper *PayloadMapper
	urlTemplate   *template.Template
}

// NewURLOutput creates a URLOutput with the given name and settings.
func NewURLOutput(name string, settings config.URLOutputConfig) (*URLOutput, error) {
	var mapper *PayloadMapper
	if settings.PayloadMapping != nil {
		mapper = NewPayloadMapper(settings.PayloadMapping)
	}

	settings.Method = strings.ToUpper(settings.Method)
	if settings.Method != "GET" && settings.Method != "POST" {
		return nil, fmt.Errorf("method must be GET or POST, got %q", settings.Method)
	}

	if err := utils.ValidateHTTPURL(settings.URL); err != nil {
		return nil, err
	}

	var tmpl *template.Template
	if strings.Contains(settings.URL, "{{") {
		var err error
		tmpl, err = template.New("url").Funcs(TemplateFuncs).Parse(settings.URL)
		if err != nil {
			return nil, fmt.Errorf("invalid URL template: %w", err)
		}
	}

	output := &URLOutput{
		OutputBase:    core.NewOutputBase(name),
		settings:      settings,
		payloadMapper: mapper,
		urlTemplate:   tmpl,
	}
	output.SetDelay(settings.Delay)
	return output, nil
}

// Send sends metadata via the configured HTTP method.
func (u *URLOutput) Send(st *core.StructuredText) {
	payload := ConvertStructuredText(st)
	u.sendRequest(payload)
}

func (u *URLOutput) sendRequest(payload *UniversalMetadata) {
	if u.settings.Method == "GET" {
		u.sendGETRequest(payload)
		return
	}
	u.sendPOSTRequest(payload)
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

func (u *URLOutput) sendGETRequest(payload *UniversalMetadata) {
	var requestURL string

	if u.urlTemplate != nil {
		templateData := payload.ToTemplateData()
		encodedData := urlEncodeTemplateData(templateData)

		var urlBuffer strings.Builder
		if err := u.urlTemplate.Execute(&urlBuffer, encodedData); err != nil {
			slog.Error("Failed to execute URL template",
				"output", u.GetName(),
				"template", u.settings.URL,
				"error", err,
			)
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

	slog.Debug("Sending GET request", //nolint:gosec // Logging URL for diagnostics
		"output", u.GetName(),
		"url", finalURL,
	)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, finalURL, http.NoBody)
	if err != nil {
		slog.Error("Failed to create GET request", "output", u.GetName(), "error", err)
		return
	}

	u.doRequest(req)
}

// doRequest sets the configured auth header, executes the request, and logs the outcome.
func (u *URLOutput) doRequest(req *http.Request) {
	if u.settings.BearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+u.settings.BearerToken)
	}

	if err := utils.DoOK(req); err != nil {
		slog.Error("Request failed", //nolint:gosec // Logging response for diagnostics
			"output", u.GetName(),
			"method", req.Method,
			"error", err,
		)
		return
	}

	slog.Debug("Successfully sent request", //nolint:gosec // Logging URL for diagnostics
		"output", u.GetName(),
		"method", req.Method,
		"url", req.URL.String(),
	)
}

func (u *URLOutput) sendPOSTRequest(payload *UniversalMetadata) {
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

	slog.Debug("Sending POST request",
		"output", u.GetName(),
		"url", u.settings.URL,
		"payload", string(jsonData),
	)

	req, err := http.NewRequestWithContext(
		context.Background(), http.MethodPost, u.settings.URL, bytes.NewBuffer(jsonData),
	)
	if err != nil {
		slog.Error("Failed to create POST request", "output", u.GetName(), "error", err)
		return
	}

	req.Header.Set("Content-Type", "application/json")
	u.doRequest(req)
}
