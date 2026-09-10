package outputs

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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
	if settings.BearerToken != "" && !strings.HasPrefix(settings.URL, "https:") {
		return nil, errors.New("bearer token requires an HTTPS URL")
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
func (u *URLOutput) Send(st *core.StructuredText) error {
	payload := ConvertStructuredText(st)
	if u.settings.Method == http.MethodGet {
		return u.sendGETRequest(payload)
	}
	return u.sendPOSTRequest(payload)
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

func (u *URLOutput) sendGETRequest(payload *UniversalMetadata) error {
	requestURL := u.settings.URL

	if u.urlTemplate != nil {
		var b strings.Builder
		if err := u.urlTemplate.Execute(&b, urlEncodeTemplateData(payload.ToTemplateData())); err != nil {
			return fmt.Errorf("execute URL template: %w", err)
		}
		requestURL = b.String()
	}

	slog.Debug("Sending GET request",
		"output", u.GetName(),
		"url", requestURL,
	)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, requestURL, http.NoBody)
	if err != nil {
		return fmt.Errorf("create GET request: %w", err)
	}

	return u.doRequest(req)
}

func (u *URLOutput) sendPOSTRequest(payload *UniversalMetadata) error {
	if u.payloadMapper != nil {
		payload.Type = "url"
	}

	jsonData, err := json.Marshal(u.payloadMapper.Apply(payload))
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
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
		return fmt.Errorf("create POST request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	return u.doRequest(req)
}

// doRequest sets the configured auth header, executes the request, and logs the outcome.
func (u *URLOutput) doRequest(req *http.Request) error {
	if u.settings.BearerToken != "" {
		if req.URL.Scheme != "https" {
			return errors.New("refusing bearer-token request over non-HTTPS URL")
		}
		req.Header.Set("Authorization", "Bearer "+u.settings.BearerToken)
	}

	if err := utils.DoOK(req); err != nil {
		return fmt.Errorf("send %s request: %w", req.Method, err)
	}

	slog.Debug("Successfully sent request",
		"output", u.GetName(),
		"method", req.Method,
		"url", req.URL.String(),
	)
	return nil
}
