package outputs

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"

	"zwfm-metadata/config"
	"zwfm-metadata/core"
	"zwfm-metadata/utils"
)

// StereoToolOutput sends metadata to StereoTool for RDS RadioText display.
type StereoToolOutput struct {
	*core.OutputBase
	core.PassiveComponent
	settings config.StereoToolOutputConfig
}

// NewStereoToolOutput creates a StereoToolOutput with the given name and settings.
func NewStereoToolOutput(name string, settings config.StereoToolOutputConfig) *StereoToolOutput {
	output := &StereoToolOutput{
		OutputBase: core.NewOutputBase(name),
		settings:   settings,
	}
	output.SetDelay(settings.Delay)
	output.SetFallbackDelay(settings.FallbackDelay)
	return output
}

// Send updates StereoTool's RadioText fields.
func (i *StereoToolOutput) Send(st *core.StructuredText) {
	if err := i.sendToStereoTool(st.String()); err != nil {
		slog.Error("Failed to update StereoTool's RadioText", "output", i.GetName(), "error", err)
	}
}

// stereoToolFields are the StereoTool parameter IDs updated with new metadata.
var stereoToolFields = []struct {
	id   int
	name string
}{
	{6751, "Streaming Output Song"},
	{15046, "FM RDS Radio Text"},
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
	requestURL := fmt.Sprintf("http://%s:%d/json-1/lis{%q:{%q:%q,%q:%q}}",
		i.settings.Hostname, i.settings.Port,
		strconv.Itoa(id), "forced", "1", "new_value", url.QueryEscape(metadata))

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, requestURL, http.NoBody)
	if err != nil {
		return fmt.Errorf("failed to create request for %s: %w", fieldName, err)
	}

	if err := utils.DoOK(req); err != nil {
		return fmt.Errorf("failed to update %s: %w", fieldName, err)
	}

	slog.Debug("Updated StereoTool field", "output", i.GetName(), "field", fieldName, "metadata", metadata)
	return nil
}
