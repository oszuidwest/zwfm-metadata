package inputs

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"zwfm-metadata/config"
	"zwfm-metadata/core"
	"zwfm-metadata/utils"
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
		httpClient: utils.CreateHTTPClient(10 * time.Second),
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
	resp, err := u.httpClient.Get(u.settings.URL)
	if err != nil {
		utils.LogError("Failed to fetch data from URL input %s: %v", u.GetName(), err)
		return
	}
	defer utils.CloseBody(resp.Body)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		utils.LogError("Failed to read response from URL input %s: %v", u.GetName(), err)
		return
	}

	var content string

	if u.settings.JSONParsing && u.settings.JSONKey != "" {
		// Parse JSON and extract key
		var data interface{}
		if err := json.Unmarshal(body, &data); err != nil {
			utils.LogError("Failed to parse JSON response from URL input %s: %v", u.GetName(), err)
			return
		}

		// Navigate through the JSON using the key path
		keys := strings.Split(u.settings.JSONKey, ".")
		current := data

		for _, key := range keys {
			switch v := current.(type) {
			case map[string]interface{}:
				if val, ok := v[key]; ok {
					current = val
				} else {
					utils.LogWarn("JSON key '%s' not found in response from URL input %s", key, u.GetName())
					return
				}
			default:
				utils.LogWarn("Cannot navigate JSON path '%s' in response from URL input %s", u.settings.JSONKey, u.GetName())
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
