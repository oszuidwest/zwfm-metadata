package inputs

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"
	"zwfm-metadata/config"
	"zwfm-metadata/core"
)

// URLInput handles URL polling input
type URLInput struct {
	*core.InputBase
	settings   config.URLInputConfig
	httpClient *http.Client
}

// NewURLInput creates a new URL input
func NewURLInput(name string, settings config.URLInputConfig) *URLInput {
	return &URLInput{
		InputBase:  core.NewInputBase(name),
		settings:   settings,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// Start implements the Input interface
func (u *URLInput) Start(ctx context.Context) error {
	ticker := time.NewTicker(time.Duration(u.settings.PollingInterval) * time.Second)
	defer ticker.Stop()

	// Poll immediately on start
	u.poll()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			u.poll()
		}
	}
}

// poll fetches data from the URL
func (u *URLInput) poll() {
	// Validate URL before making request
	parsedURL, err := url.Parse(u.settings.URL)
	if err != nil {
		slog.Error("Invalid URL in configuration", "input", u.GetName(), "url", u.settings.URL, "error", err)
		return
	}

	// Ensure URL has a valid scheme
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		slog.Error("URL must use http or https scheme", "input", u.GetName(), "url", u.settings.URL, "scheme", parsedURL.Scheme)
		return
	}

	resp, err := u.httpClient.Get(u.settings.URL)
	if err != nil {
		slog.Error("Failed to fetch data from URL input", "input", u.GetName(), "error", err)
		return
	}
	defer resp.Body.Close() //nolint:errcheck

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.Error("Failed to read response from URL input", "input", u.GetName(), "error", err)
		return
	}

	var content string

	if u.settings.JSONParsing && u.settings.JSONKey != "" {
		// Parse JSON and extract key
		var data interface{}
		if err := json.Unmarshal(body, &data); err != nil {
			slog.Error("Failed to parse JSON response", "input", u.GetName(), "error", err)
			return
		}

		// Navigate through the JSON using the key path
		current := data
		for _, key := range strings.Split(u.settings.JSONKey, ".") {
			m, ok := current.(map[string]interface{})
			if !ok {
				slog.Error("Cannot navigate JSON path", "input", u.GetName(), "path", u.settings.JSONKey)
				return
			}
			current, ok = m[key]
			if !ok {
				slog.Error("Key not found in JSON", "input", u.GetName(), "key", key)
				return
			}
		}

		content = fmt.Sprintf("%v", current)
	} else {
		content = string(body)
	}

	metadata := &core.Metadata{
		Name:      u.GetName(),
		Title:     content,
		UpdatedAt: time.Now(),
	}

	// Set metadata (SetMetadata will handle change detection)
	u.SetMetadata(metadata)
}
