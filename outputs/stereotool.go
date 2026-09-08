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

const (
	stereoTool11SongFieldID      = 6751
	stereoTool11RadioTextFieldID = 9985
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

type stereoToolField struct {
	id   int
	name string
}

func (i *StereoToolOutput) sendToStereoTool(metadata string) error {
	fields := [...]stereoToolField{
		{id: stereoTool11SongFieldID, name: "Streaming Output Song"},
		{id: stereoTool11RadioTextFieldID, name: "FM RDS Radio Text"},
	}

	for _, field := range fields {
		if err := i.updateField(field.id, field.name, metadata); err != nil {
			return err
		}
	}
	return nil
}

func (i *StereoToolOutput) updateField(id int, fieldName, metadata string) error {
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

	escapedPayload := url.QueryEscape(strings.TrimSuffix(payload.String(), "\n"))
	escapedPayload = strings.ReplaceAll(escapedPayload, "+", "%20")
	requestURL := fmt.Sprintf(
		"http://%s:%d/json-1/lis%s",
		i.settings.Hostname,
		i.settings.Port,
		escapedPayload,
	)

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
