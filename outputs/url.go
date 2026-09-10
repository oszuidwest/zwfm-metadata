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
	mapper, err := NewPayloadMapper(settings.PayloadMapping)
	if err != nil {
		return nil, err
	}

	settings.Method = strings.ToUpper(settings.Method)
	if settings.Method != http.MethodGet && settings.Method != http.MethodPost {
		return nil, fmt.Errorf("method must be GET or POST, got %q", settings.Method)
	}

	if err := utils.ValidateHTTPURL(settings.URL); err != nil {
		return nil, err
	}

	var tmpl *template.Template
	if isTemplate(settings.URL) {
		tmpl, err = template.New("url").Funcs(templateFuncs).Parse(settings.URL)
		if err != nil {
			return nil, fmt.Errorf("invalid URL template: %w", err)
		}
	}

	return &URLOutput{
		OutputBase:    core.NewOutputBase(name),
		settings:      settings,
		payloadMapper: mapper,
		urlTemplate:   tmpl,
	}, nil
}

// Send sends metadata via the configured HTTP method.
func (u *URLOutput) Send(st *core.StructuredText) {
	payload := ConvertStructuredText(st)
	if u.settings.Method == http.MethodGet {
		u.sendGETRequest(payload)
		return
	}
	u.sendPOSTRequest(payload)
}

// urlEncodeTemplateData query-escapes every string so templates can splice values into a URL.
func urlEncodeTemplateData(data map[string]any) map[string]any {
	encoded := make(map[string]any, len(data))
	for key, value := range data {
		if s, ok := value.(string); ok {
			encoded[key] = url.QueryEscape(s)
		} else {
			encoded[key] = value
		}
	}
	return encoded
}

func (u *URLOutput) sendGETRequest(payload *UniversalMetadata) {
	requestURL := u.settings.URL

	if u.urlTemplate != nil {
		var b strings.Builder
		if err := u.urlTemplate.Execute(&b, urlEncodeTemplateData(payload.ToTemplateData())); err != nil {
			slog.Error("Failed to execute URL template",
				"output", u.GetName(),
				"template", u.settings.URL,
				"error", err,
			)
			return
		}
		requestURL = b.String()
	}

	slog.Debug("Sending GET request", //nolint:gosec // Logging URL for diagnostics
		"output", u.GetName(),
		"url", requestURL,
	)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, requestURL, http.NoBody)
	if err != nil {
		slog.Error("Failed to create GET request", "output", u.GetName(), "error", err)
		return
	}

	u.doRequest(req)
}

func (u *URLOutput) sendPOSTRequest(payload *UniversalMetadata) {
	if u.payloadMapper != nil {
		payload.Type = "url"
	}

	jsonData, err := json.Marshal(u.payloadMapper.Apply(payload))
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
		context.Background(), http.MethodPost, u.settings.URL, bytes.NewReader(jsonData),
	)
	if err != nil {
		slog.Error("Failed to create POST request", "output", u.GetName(), "error", err)
		return
	}

	req.Header.Set("Content-Type", "application/json")
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
