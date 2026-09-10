package inputs

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"zwfm-metadata/config"
	"zwfm-metadata/core"
	"zwfm-metadata/utils"
)

// URLInput polls an external URL for metadata with optional JSON parsing.
type URLInput struct {
	*core.InputBase
	settings config.URLInputConfig
}

// NewURLInput creates a URLInput with the given name and settings.
func NewURLInput(name string, settings *config.URLInputConfig) (*URLInput, error) {
	if err := utils.ValidateHTTPURL(settings.URL); err != nil {
		return nil, err
	}
	if settings.PollingInterval < 1 {
		return nil, fmt.Errorf("pollingInterval must be at least 1 second, got %d", settings.PollingInterval)
	}

	return &URLInput{
		InputBase: core.NewInputBase(name),
		settings:  *settings,
	}, nil
}

// Start polls on the configured interval, and additionally as soon as the current
// metadata expires, until context cancellation.
func (u *URLInput) Start(ctx context.Context) error {
	polls := time.Tick(time.Duration(u.settings.PollingInterval) * time.Second)

	expiry := time.NewTimer(0)
	expiry.Stop()

	for {
		u.poll()
		if metadata := u.GetMetadata(); metadata != nil && metadata.ExpiresAt != nil {
			if until := time.Until(*metadata.ExpiresAt); until > 0 {
				expiry.Reset(until)
			} else {
				expiry.Stop()
			}
		} else {
			expiry.Stop()
		}

		select {
		case <-ctx.Done():
			return nil
		case <-polls:
		case <-expiry.C:
		}
	}
}

func (u *URLInput) poll() {
	resp, err := utils.Get(context.Background(), u.settings.URL)
	if err != nil {
		slog.Error("Failed to fetch data from URL input", "input", u.GetName(), "error", err)
		return
	}
	defer resp.Body.Close() //nolint:errcheck // Best-effort cleanup

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.Error("Failed to read response from URL input", "input", u.GetName(), "error", err)
		return
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		slog.Error("URL input returned unsuccessful status", "input", u.GetName(), "status", resp.Status)
		return
	}

	metadata := &core.Metadata{
		Title:     string(body),
		UpdatedAt: time.Now(),
	}

	if u.settings.JSONParsing && u.settings.JSONKey != "" {
		var ok bool
		metadata.Title, metadata.ExpiresAt, ok = u.parseJSON(body)
		if !ok {
			return
		}
	}

	u.SetMetadata(metadata)
}

// parseJSON extracts the title and optional expiry from a JSON body. It reports
// false when the title cannot be found; a bad expiry is logged and ignored.
func (u *URLInput) parseJSON(body []byte) (title string, expiresAt *time.Time, ok bool) {
	var data any
	if err := json.Unmarshal(body, &data); err != nil {
		slog.Error("Failed to parse JSON response", "input", u.GetName(), "error", err)
		return "", nil, false
	}

	titleVal, found := extractJSONValue(data, u.settings.JSONKey)
	if !found {
		slog.Error("Cannot navigate JSON path", "input", u.GetName(), "path", u.settings.JSONKey)
		return "", nil, false
	}
	title = fmt.Sprint(titleVal)

	if u.settings.ExpiryKey == "" {
		return title, nil, true
	}

	expVal, found := extractJSONValue(data, u.settings.ExpiryKey)
	if !found {
		slog.Error("Cannot navigate expiry JSON path", "input", u.GetName(), "path", u.settings.ExpiryKey)
		return title, nil, true
	}
	expStr, isString := expVal.(string)
	if !isString {
		slog.Error("Expiry value is not a string", "input", u.GetName(), "value", expVal)
		return title, nil, true
	}

	t, err := time.Parse(cmp.Or(u.settings.ExpiryFormat, time.RFC3339), expStr)
	if err != nil {
		slog.Error("Failed to parse expiry time", "input", u.GetName(), "value", expStr, "error", err)
		return title, nil, true
	}
	return title, &t, true
}

// extractJSONValue navigates a JSON structure using a dot-separated key path.
func extractJSONValue(data any, keyPath string) (any, bool) {
	current := data
	for key := range strings.SplitSeq(keyPath, ".") {
		m, ok := current.(map[string]any)
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
