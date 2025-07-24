package outputs

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"time"
	"zwfm-metadata/config"
	"zwfm-metadata/core"
)

// StereoToolOutput handles sending metadata to StereoTool's RadioText
type StereoToolOutput struct {
	*core.OutputBase
	core.PassiveComponent
	settings   config.StereoToolOutputConfig
	httpClient *http.Client
}

// NewStereoToolOutput creates a new StereoTool output
func NewStereoToolOutput(name string, settings config.StereoToolOutputConfig) *StereoToolOutput {
	output := &StereoToolOutput{
		OutputBase: core.NewOutputBase(name),
		settings:   settings,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
	output.SetDelay(settings.Delay)
	return output
}

// SendFormattedMetadata implements the Output interface (called by metadata router)
func (i *StereoToolOutput) SendFormattedMetadata(formattedText string) {
	// Check if value changed to avoid unnecessary HTTP requests
	if !i.HasChanged(formattedText) {
		return
	}

	// Update StereoTool's RadioText
	if err := i.sendToStereoTool(formattedText); err != nil {
		slog.Error("Failed to update StereoTool's RadioText", "output", i.GetName(), "error", err)
	}
}

// sendToStereoTool sends the metadata to StereoTool's RadioText
func (i *StereoToolOutput) sendToStereoTool(metadata string) error {
	fieldNames := map[int]string{
		6751: "Streaming Output Song",
		9985: "FM RDS Radio Text",
	}

	for id, fieldName := range fieldNames {
		requestURL := fmt.Sprintf("http://%s:%d/json-1/lis{%q:{%q:%q,%q:%q}}",
			i.settings.Hostname, i.settings.Port,
			fmt.Sprintf("%d", id), "forced", "1", "new_value", url.QueryEscape(metadata))

		resp, err := i.httpClient.Get(requestURL)
		if err != nil {
			return fmt.Errorf("failed to update %s: %w", fieldName, err)
		}
		resp.Body.Close() //nolint:errcheck

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("StereoTool API error for %s: status %d", fieldName, resp.StatusCode)
		}

		slog.Debug("Updated StereoTool field", "field", fieldName, "metadata", metadata)
	}
	return nil
}
