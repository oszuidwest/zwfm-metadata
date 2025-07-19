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
	expiresAt  *time.Time
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

	var expiryTimer *time.Timer

	// Poll immediately on start
	u.poll()
	u.updateExpiryTimer(&expiryTimer)

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			u.poll()
			u.updateExpiryTimer(&expiryTimer)
		case <-u.expiryTimerChan(expiryTimer):
			u.poll()
			u.updateExpiryTimer(&expiryTimer)
		}
	}
}

// Helper to update expiry timer
func (u *URLInput) updateExpiryTimer(timer **time.Timer) {
	if u.expiresAt != nil {
		duration := time.Until(*u.expiresAt)
		if duration > 0 {
			if *timer != nil {
				(*timer).Reset(duration)
			} else {
				*timer = time.NewTimer(duration)
			}
		}
	}
}

// Helper to get channel for expiry timer
func (u *URLInput) expiryTimerChan(timer *time.Timer) <-chan time.Time {
	if timer != nil {
		return timer.C
	}
	return make(chan time.Time) // never fires
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
	var expiresAt *time.Time

	if u.settings.JSONParsing && u.settings.JSONKey != "" {
		// Parse JSON and extract key
		var data interface{}
		if err := json.Unmarshal(body, &data); err != nil {
			slog.Error("Failed to parse JSON response", "input", u.GetName(), "error", err)
			return
		}

		// Extract main content
		contentVal, ok := extractJSONValue(data, u.settings.JSONKey)
		if !ok {
			slog.Error("Cannot navigate JSON path", "input", u.GetName(), "path", u.settings.JSONKey)
			return
		}
		content = fmt.Sprintf("%v", contentVal)

		// Extract expiry if configured
		if u.settings.ExpiryKey != "" {
			expVal, ok := extractJSONValue(data, u.settings.ExpiryKey)
			if !ok {
				slog.Error("Cannot navigate expiry JSON path", "input", u.GetName(), "path", u.settings.ExpiryKey)
			} else if expStr, ok := expVal.(string); ok {
				var t time.Time
				var err error
				if u.settings.ExpiryFormat != "" {
					t, err = time.Parse(u.settings.ExpiryFormat, expStr)
				} else {
					t, err = time.Parse(time.RFC3339, expStr)
				}
				if err != nil {
					slog.Error("Failed to parse expiry time", "input", u.GetName(), "value", expStr, "error", err)
				} else {
					expiresAt = &t
				}
			}
		}
	} else {
		content = string(body)
	}

	metadata := &core.Metadata{
		Name:      u.GetName(),
		Title:     content,
		UpdatedAt: time.Now(),
		ExpiresAt: expiresAt,
	}

	u.expiresAt = expiresAt

	u.SetMetadata(metadata)
}

// extractJSONValue navigates a JSON structure using a dot-separated key path
func extractJSONValue(data interface{}, keyPath string) (interface{}, bool) {
	current := data
	for _, key := range strings.Split(keyPath, ".") {
		m, ok := current.(map[string]interface{})
		if !ok {
			return nil, false
		}
		current, ok = m[key]
		if !ok {
			return nil, false
		}
	}
	return current, true
}
