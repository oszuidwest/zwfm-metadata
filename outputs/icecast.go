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

// IcecastOutput sends metadata updates to Icecast streaming servers.
type IcecastOutput struct {
	*core.OutputBase
	core.PassiveComponent
	settings   config.IcecastOutputConfig
	httpClient *http.Client
}

// NewIcecastOutput creates an IcecastOutput with the given name and settings.
func NewIcecastOutput(name string, settings *config.IcecastOutputConfig) *IcecastOutput {
	output := &IcecastOutput{
		OutputBase: core.NewOutputBase(name),
		settings:   *settings,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
	output.SetDelay(settings.Delay)
	return output
}

// Send updates the Icecast server with new metadata.
func (i *IcecastOutput) Send(st *core.StructuredText) {
	text := st.String()
	if !i.HasChanged(text) {
		return
	}

	if err := i.sendToIcecast(text); err != nil {
		slog.Error("Failed to update Icecast server", "output", i.GetName(), "error", err)
	}
}

func (i *IcecastOutput) sendToIcecast(metadata string) error {
	baseURL := fmt.Sprintf("http://%s:%d/admin/metadata", i.settings.Server, i.settings.Port)

	params := url.Values{}
	params.Set("mount", i.settings.Mountpoint)
	params.Set("mode", "updinfo")
	params.Set("song", metadata)
	params.Set("charset", "UTF-8")

	fullURL := baseURL + "?" + params.Encode()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, http.NoBody)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.SetBasicAuth(i.settings.Username, i.settings.Password)
	req.Header.Set("Content-Type", "text/plain; charset=utf-8")

	resp, err := i.httpClient.Do(req)
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
