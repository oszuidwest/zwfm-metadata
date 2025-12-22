package outputs

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"zwfm-metadata/config"
	"zwfm-metadata/core"
)

// StereoToolOutput sends metadata to StereoTool for RDS RadioText display.
type StereoToolOutput struct {
	*core.OutputBase
	core.PassiveComponent
	settings   config.StereoToolOutputConfig
	httpClient *http.Client
}

// NewStereoToolOutput creates a StereoToolOutput with the given name and settings.
func NewStereoToolOutput(name string, settings config.StereoToolOutputConfig) *StereoToolOutput {
	output := &StereoToolOutput{
		OutputBase: core.NewOutputBase(name),
		settings:   settings,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
	output.SetDelay(settings.Delay)
	return output
}

// Send updates StereoTool's RadioText fields.
func (i *StereoToolOutput) Send(st *core.StructuredText) {
	text := st.String()
	if !i.HasChanged(text) {
		return
	}

	if err := i.sendToStereoTool(text); err != nil {
		slog.Error("Failed to update StereoTool's RadioText", "output", i.GetName(), "error", err)
	}
}

func (i *StereoToolOutput) sendToStereoTool(metadata string) error {
	fieldNames := map[int]string{
		6751:  "Streaming Output Song",
		15046: "FM RDS Radio Text",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	for id, fieldName := range fieldNames {
		requestURL := fmt.Sprintf("http://%s:%d/json-1/lis{%q:{%q:%q,%q:%q}}",
			i.settings.Hostname, i.settings.Port,
			fmt.Sprintf("%d", id), "forced", "1", "new_value", url.QueryEscape(metadata))

		req, err := http.NewRequestWithContext(ctx, "GET", requestURL, nil)
		if err != nil {
			return fmt.Errorf("failed to create request for %s: %w", fieldName, err)
		}

		resp, err := i.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("failed to update %s: %w", fieldName, err)
		}
		defer resp.Body.Close() //nolint:errcheck

		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("StereoTool API error for %s: status %d, response: %s", fieldName, resp.StatusCode, string(bodyBytes))
		}

		slog.Debug("Updated StereoTool field", "output", i.GetName(), "field", fieldName, "metadata", metadata)
	}
	return nil
}
