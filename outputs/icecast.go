package outputs

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"

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
	return &IcecastOutput{OutputBase: core.NewOutputBase(name), settings: *settings}
}

// Send updates the Icecast server with new metadata.
func (i *IcecastOutput) Send(st *core.StructuredText) error {
	return i.sendToIcecast(st.String())
}

func (i *IcecastOutput) sendToIcecast(metadata string) error {
	scheme := "http"
	if i.settings.TLS {
		scheme = "https"
	}
	reqURL := &url.URL{
		Scheme: scheme,
		Host:   joinHostPort(i.settings.Server, i.settings.Port),
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

	body, err := utils.DoOKBody(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	// Icecast also answers a refused update with 200 and <return>1</return>.
	if !bytes.Contains(body, []byte("Metadata update successful")) {
		return fmt.Errorf("icecast refused the update: %s", body)
	}

	slog.Debug("Successfully updated Icecast", "output", i.GetName(), "metadata", metadata)

	return nil
}

func joinHostPort(host string, port int) string {
	return net.JoinHostPort(strings.Trim(host, "[]"), strconv.Itoa(port))
}
