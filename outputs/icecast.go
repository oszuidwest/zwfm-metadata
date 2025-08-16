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

// IcecastOutput handles sending metadata to Icecast servers.
type IcecastOutput struct {
	*core.OutputBase
	core.PassiveComponent
	settings   config.IcecastOutputConfig
	httpClient *http.Client
}

// NewIcecastOutput creates a new Icecast output.
func NewIcecastOutput(name string, settings config.IcecastOutputConfig) *IcecastOutput {
	output := &IcecastOutput{
		OutputBase: core.NewOutputBase(name),
		settings:   settings,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
	output.SetDelay(settings.Delay)
	return output
}

// SendFormattedMetadata implements the Output interface (called by metadata router).
func (i *IcecastOutput) SendFormattedMetadata(formattedText string) {
	// Check if value changed to avoid unnecessary HTTP requests
	if !i.HasChanged(formattedText) {
		return
	}

	// Update Icecast
	if err := i.sendToIcecast(formattedText); err != nil {
		slog.Error("Failed to update Icecast server", "output", i.GetName(), "error", err)
	}
}

// sendToIcecast sends the metadata to the Icecast server.
func (i *IcecastOutput) sendToIcecast(metadata string) error {
	// Build URL
	baseURL := fmt.Sprintf("http://%s:%d/admin/metadata", i.settings.Server, i.settings.Port)

	// Prepare parameters
	params := url.Values{}
	params.Set("mount", i.settings.Mountpoint)
	params.Set("mode", "updinfo")
	params.Set("song", metadata)
	params.Set("charset", "UTF-8")

	fullURL := baseURL + "?" + params.Encode()

	// Create request
	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Add authentication
	req.SetBasicAuth(i.settings.Username, i.settings.Password)
	req.Header.Set("Content-Type", "text/plain; charset=utf-8")

	// Send request
	resp, err := i.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	slog.Debug("Successfully updated Icecast", "output", i.GetName(), "metadata", metadata)

	return nil
}
