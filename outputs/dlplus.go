package outputs

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"unicode/utf8"
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

// NewDLPlusOutput creates a DLPlusOutput with the given name and settings.
func NewDLPlusOutput(name string, settings config.DLPlusOutputConfig) *DLPlusOutput {
	output := &DLPlusOutput{
		OutputBase: core.NewOutputBase(name),
		settings:   settings,
	}
	output.SetDelay(settings.Delay)
	return output
}

// SendFormattedMetadata is unused; DLPlusOutput implements EnhancedOutput.
func (o *DLPlusOutput) SendFormattedMetadata(_ string) {}

// SendEnhancedMetadata writes metadata with DL Plus tags to the configured file.
func (o *DLPlusOutput) SendEnhancedMetadata(formattedText string, metadata *core.Metadata, inputName, inputType string) {
	if !o.HasChanged(formattedText) {
		return
	}

	content := o.buildDLPlusContent(formattedText, metadata)

	if err := o.writeToFile(content); err != nil {
		slog.Error("Failed to write DL Plus file", "output", o.GetName(), "filename", o.settings.Filename, "error", err)
		return
	}

	slog.Debug("Wrote DL Plus", "output", o.GetName(), "filename", o.settings.Filename)
}

func (o *DLPlusOutput) buildDLPlusContent(formattedText string, metadata *core.Metadata) string {
	var content strings.Builder

	content.WriteString("##### parameters { #####\n")
	content.WriteString("DL_PLUS=1\n")

	isRunning := metadata.Artist != "" && metadata.Title != ""

	o.toggleValue = !o.toggleValue
	toggleInt := 0
	if o.toggleValue {
		toggleInt = 1
	}

	o.addDLPlusTags(&content, formattedText, metadata)

	runningInt := 0
	if isRunning {
		runningInt = 1
	}
	fmt.Fprintf(&content, "DL_PLUS_ITEM_RUNNING=%d\n", runningInt)
	fmt.Fprintf(&content, "DL_PLUS_ITEM_TOGGLE=%d\n", toggleInt)

	content.WriteString("##### parameters } #####\n")
	content.WriteString(formattedText)

	return content.String()
}

func (o *DLPlusOutput) addDLPlusTags(content *strings.Builder, formattedText string, metadata *core.Metadata) {
	if metadata.Artist != "" {
		if bytePos := strings.Index(formattedText, metadata.Artist); bytePos >= 0 {
			runePos := utf8.RuneCountInString(formattedText[:bytePos])
			length := utf8.RuneCountInString(metadata.Artist) - 1
			if length >= 0 {
				fmt.Fprintf(content, "DL_PLUS_TAG=%d %d %d\n", dlPlusTypeArtist, runePos, length)
			}
		}
	}

	if metadata.Title != "" {
		if bytePos := strings.Index(formattedText, metadata.Title); bytePos >= 0 {
			runePos := utf8.RuneCountInString(formattedText[:bytePos])
			length := utf8.RuneCountInString(metadata.Title) - 1
			if length >= 0 {
				fmt.Fprintf(content, "DL_PLUS_TAG=%d %d %d\n", dlPlusTypeTitle, runePos, length)
			}
		}
	}
}

func (o *DLPlusOutput) writeToFile(content string) error {
	return utils.WriteFile(o.settings.Filename, []byte(content))
}

// Start creates the initial empty output file.
func (o *DLPlusOutput) Start(_ context.Context) error {
	slog.Info("DL Plus output writing to file", "output", o.GetName(), "filename", o.settings.Filename)

	if err := utils.WriteFile(o.settings.Filename, []byte("")); err != nil {
		return fmt.Errorf("failed to create DL Plus file: %w", err)
	}

	return nil
}

// Stop performs cleanup when the output shuts down.
func (o *DLPlusOutput) Stop() error {
	slog.Debug("Stopped DL Plus output", "output", o.GetName(), "filename", o.settings.Filename)
	return nil
}
