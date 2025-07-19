package outputs

import (
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
	// Build JSON payload
	radioTextId := 9985
	jsonPayload := fmt.Sprintf(`{"%d":{"forced":"1", "new_value":"%s"}}`, radioTextId, metadata)

	// URL-encode the JSON payload
	encodedPayload := url.QueryEscape(jsonPayload)

	// Build full URL (append encoded JSON)
	fullURL := fmt.Sprintf("http://%s:%d/json-1/lis%s", i.settings.Hostname, i.settings.Port, encodedPayload)

	slog.Debug("Updating StereoTool's RadioText", "fullurl", fullURL, "metadata", metadata)

	// Create request
	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Send request
	resp, err := i.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Check response for radioTextId
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}
	bodyStr := string(bodyBytes)
	expectedValueText := fmt.Sprintf(`"value_text": "%s"`, metadata)
	if !strings.Contains(bodyStr, expectedValueText) {
		return fmt.Errorf("response didn't indicate a succesfull update")
	}

	slog.Debug("Successfully updated StereoTool's RadioText", "output", i.GetName(), "metadata", metadata)

	return nil
}
