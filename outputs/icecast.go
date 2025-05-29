package outputs

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"time"
	"zwfm-metadata/config"
	"zwfm-metadata/core"
	"zwfm-metadata/utils"
)

// IcecastOutput handles sending metadata to Icecast servers
type IcecastOutput struct {
	*core.BaseOutput
	core.WaitForShutdown
	settings   config.IcecastOutputSettings
	httpClient *http.Client
}

// NewIcecastOutput creates a new Icecast output
func NewIcecastOutput(name string, settings config.IcecastOutputSettings) *IcecastOutput {
	return &IcecastOutput{
		BaseOutput: core.NewBaseOutput(name),
		settings:   settings,
		httpClient: utils.CreateHTTPClient(10 * time.Second),
	}
}

// GetDelay implements the Output interface
func (i *IcecastOutput) GetDelay() int {
	return i.settings.Delay
}

// ProcessFormattedMetadata implements the Output interface (called by timeline manager)
func (i *IcecastOutput) ProcessFormattedMetadata(formattedText string) {
	// Check if value changed to avoid unnecessary HTTP requests
	if !i.HasChanged(formattedText) {
		return
	}

	// Update Icecast
	if err := i.sendToIcecast(formattedText); err != nil {
		utils.LogError("Failed to update Icecast server from output %s: %v", i.GetName(), err)
	}
}

// sendToIcecast sends the metadata to the Icecast server
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
	auth := base64.StdEncoding.EncodeToString([]byte(i.settings.Username + ":" + i.settings.Password))
	req.Header.Set("Authorization", "Basic "+auth)
	req.Header.Set("Content-Type", "text/plain; charset=utf-8")

	// Send request
	resp, err := i.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer utils.CloseBody(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	utils.LogDebug("Successfully updated Icecast %s with: %s", i.GetName(), metadata)

	return nil
}
