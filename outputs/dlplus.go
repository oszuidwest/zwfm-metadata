package outputs

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"zwfm-metadata/config"
	"zwfm-metadata/core"
	"zwfm-metadata/utils"
)

const (
	dlPlusTypeTitle  = 1
	dlPlusTypeArtist = 4
)

// DLPlusOutput writes DAB/DAB+ Dynamic Label Plus formatted metadata for ODR-PadEnc.
type DLPlusOutput struct {
	*core.OutputBase
	core.PassiveComponent
	settings    config.DLPlusOutputConfig
	toggleValue bool // Alternates between true/false to indicate content changes
}

// NewDLPlusOutput initializes a DL Plus file writer with the given settings.
func NewDLPlusOutput(name string, settings config.DLPlusOutputConfig) *DLPlusOutput {
	output := &DLPlusOutput{
		OutputBase: core.NewOutputBase(name),
		settings:   settings,
	}
	output.SetDelay(settings.Delay)
	return output
}

// Send writes metadata with DL Plus tags to the configured file.
func (o *DLPlusOutput) Send(st *core.StructuredText) {
	text := st.String()
	if !o.HasChanged(text) {
		return
	}

	content := o.buildDLPlusContent(st)

	if err := utils.WriteFile(o.settings.Filename, []byte(content)); err != nil {
		slog.Error("Failed to write DL Plus file", "output", o.GetName(), "filename", o.settings.Filename, "error", err)
		return
	}

	slog.Debug("Wrote DL Plus", "output", o.GetName(), "filename", o.settings.Filename)
}

func (o *DLPlusOutput) buildDLPlusContent(st *core.StructuredText) string {
	var content strings.Builder

	content.WriteString("##### parameters { #####\n")
	content.WriteString("DL_PLUS=1\n")

	o.toggleValue = !o.toggleValue
	toggleInt := 0
	if o.toggleValue {
		toggleInt = 1
	}

	o.addDLPlusTags(&content, st)

	runningInt := 0
	if st.IsRunning() {
		runningInt = 1
	}

	fmt.Fprintf(&content, "DL_PLUS_ITEM_RUNNING=%d\n", runningInt)
	fmt.Fprintf(&content, "DL_PLUS_ITEM_TOGGLE=%d\n", toggleInt)

	content.WriteString("##### parameters } #####\n")
	content.WriteString(st.String())

	return content.String()
}

func (o *DLPlusOutput) addDLPlusTags(content *strings.Builder, st *core.StructuredText) {
	if start, length, ok := st.ArtistRange(); ok && length >= 0 {
		fmt.Fprintf(content, "DL_PLUS_TAG=%d %d %d\n", dlPlusTypeArtist, start, length)
	}

	if start, length, ok := st.TitleRange(); ok && length >= 0 {
		fmt.Fprintf(content, "DL_PLUS_TAG=%d %d %d\n", dlPlusTypeTitle, start, length)
	}
}

// Start creates an empty output file for ODR-PadEnc to monitor.
func (o *DLPlusOutput) Start(_ context.Context) error {
	slog.Info("DL Plus output writing to file", "output", o.GetName(), "filename", o.settings.Filename)

	if err := utils.WriteFile(o.settings.Filename, []byte("")); err != nil {
		return fmt.Errorf("failed to create DL Plus file: %w", err)
	}

	return nil
}

// Stop handles graceful shutdown of the DL Plus output.
func (o *DLPlusOutput) Stop() error {
	slog.Debug("Stopped DL Plus output", "output", o.GetName(), "filename", o.settings.Filename)
	return nil
}
