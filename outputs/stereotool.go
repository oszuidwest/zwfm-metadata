package outputs

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"zwfm-metadata/config"
	"zwfm-metadata/core"
	"zwfm-metadata/utils"
)

// StereoToolOutput sends RDS and streaming metadata to Stereo Tool.
type StereoToolOutput struct {
	*core.OutputBase
	core.PassiveComponent
	settings config.StereoToolOutputConfig
}

// NewStereoToolOutput creates a Stereo Tool metadata output.
func NewStereoToolOutput(name string, settings config.StereoToolOutputConfig) *StereoToolOutput {
	return &StereoToolOutput{OutputBase: core.NewOutputBase(name), settings: settings}
}

// Send updates Stereo Tool's RDS and streaming metadata fields.
func (i *StereoToolOutput) Send(st *core.StructuredText) {
	if err := i.sendToStereoTool(st.String()); err != nil {
		slog.Error("Failed to update Stereo Tool metadata", "output", i.GetName(), "error", err)
	}
}

// Stereo Tool 10.75 and 11.05 use these metadata field IDs.
var stereoToolFields = []struct {
	id   int
	name string
}{
	{6751, "Streaming Output Song"},
	{9985, "FM RDS Radio Text"},
}

func (i *StereoToolOutput) sendToStereoTool(metadata string) error {
	for _, field := range stereoToolFields {
		if err := i.updateField(field.id, field.name, metadata); err != nil {
			return err
		}
	}
	return nil
}

func (i *StereoToolOutput) updateField(id int, fieldName, metadata string) error {
	// json.Marshal would escape & as \u0026; Stereo Tool's undocumented parser gets
	// the literal character, as it always has.
	var payload bytes.Buffer
	encoder := json.NewEncoder(&payload)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(map[string]map[string]string{
		strconv.Itoa(id): {
			"forced":    "1",
			"new_value": metadata,
		},
	}); err != nil {
		return fmt.Errorf("failed to encode request for %s: %w", fieldName, err)
	}

	// Stereo Tool needs every reserved character percent-encoded (url.PathEscape would
	// leave & and + literal) and spaces as %20 rather than QueryEscape's +.
	escapedPayload := strings.ReplaceAll(url.QueryEscape(strings.TrimSuffix(payload.String(), "\n")), "+", "%20")
	requestURL := fmt.Sprintf("http://%s:%d/json-1/lis%s", i.settings.Hostname, i.settings.Port, escapedPayload)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, requestURL, http.NoBody)
	if err != nil {
		return fmt.Errorf("failed to create request for %s: %w", fieldName, err)
	}

	if err := utils.DoOK(req); err != nil {
		return fmt.Errorf("failed to update %s: %w", fieldName, err)
	}

	slog.Debug("Updated Stereo Tool field", "output", i.GetName(), "field", fieldName, "metadata", metadata)
	return nil
}
