package outputs

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"

	"zwfm-metadata/config"
	"zwfm-metadata/core"
	"zwfm-metadata/utils"
)

// IcecastOutput sends metadata updates to Icecast streaming servers.
type IcecastOutput struct {
	*core.OutputBase
	core.PassiveComponent
	settings config.IcecastOutputConfig
}

// NewIcecastOutput creates an IcecastOutput with the given name and settings.
func NewIcecastOutput(name string, settings *config.IcecastOutputConfig) *IcecastOutput {
	output := &IcecastOutput{
		OutputBase: core.NewOutputBase(name),
		settings:   *settings,
	}
	output.SetDelay(settings.Delay)
	return output
}

// Send updates the Icecast server with new metadata.
func (i *IcecastOutput) Send(st *core.StructuredText) {
	if err := i.sendToIcecast(st.String()); err != nil {
		slog.Error("Failed to update Icecast server", "output", i.GetName(), "error", err)
	}
}

func (i *IcecastOutput) sendToIcecast(metadata string) error {
	reqURL := &url.URL{
		Scheme: "http",
		Host:   fmt.Sprintf("%s:%d", i.settings.Server, i.settings.Port),
		Path:   "/admin/metadata",
	}

	params := url.Values{}
	params.Set("mount", i.settings.Mountpoint)
	params.Set("mode", "updinfo")
	params.Set("song", metadata)
	params.Set("charset", "UTF-8")
	reqURL.RawQuery = params.Encode()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, reqURL.String(), http.NoBody)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.SetBasicAuth(i.settings.Username, i.settings.Password)
	req.Header.Set("Content-Type", "text/plain; charset=utf-8")

	resp, err := utils.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // Best-effort cleanup

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status code: %d, response: %s", resp.StatusCode, string(bodyBytes))
	}

	slog.Debug("Successfully updated Icecast", "output", i.GetName(), "metadata", metadata)

	return nil
}
