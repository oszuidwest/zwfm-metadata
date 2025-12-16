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

// Content type constants for DL Plus tags
const (
	dlPlusTypeTitle  = 1
	dlPlusTypeArtist = 4
)

// DLPlusOutput writes metadata in DL Plus format for ODR-PadEnc.
// It implements EnhancedOutput to access raw metadata fields.
type DLPlusOutput struct {
	*core.OutputBase
	core.PassiveComponent
	settings    config.DLPlusOutputConfig
	toggleValue bool // Alternates between true/false to indicate content changes
}

// NewDLPlusOutput creates a new DL Plus output instance.
func NewDLPlusOutput(name string, settings config.DLPlusOutputConfig) *DLPlusOutput {
	output := &DLPlusOutput{
		OutputBase: core.NewOutputBase(name),
		settings:   settings,
	}
	output.SetDelay(settings.Delay)
	return output
}

// SendFormattedMetadata is not used for DL Plus output.
func (o *DLPlusOutput) SendFormattedMetadata(_ string) {
	// This won't be called since we implement EnhancedOutput
}

// SendEnhancedMetadata writes the metadata in DL Plus format.
func (o *DLPlusOutput) SendEnhancedMetadata(formattedText string, metadata *core.Metadata, inputName, inputType string) {
	// Check if value changed to avoid unnecessary file writes
	if !o.HasChanged(formattedText) {
		return
	}

	// Build the DL Plus content
	content := o.buildDLPlusContent(formattedText, metadata)

	// Write to file
	if err := o.writeToFile(content); err != nil {
		slog.Error("Failed to write DL Plus file", "output", o.GetName(), "filename", o.settings.Filename, "error", err)
		return
	}

	slog.Debug("Wrote DL Plus", "output", o.GetName(), "filename", o.settings.Filename)
}

// buildDLPlusContent creates the DL Plus formatted content.
func (o *DLPlusOutput) buildDLPlusContent(formattedText string, metadata *core.Metadata) string {
	var content strings.Builder

	// Write parameter block header
	content.WriteString("##### parameters { #####\n")
	content.WriteString("DL_PLUS=1\n")

	// Determine if this is running content (has both artist and title)
	isRunning := metadata.Artist != "" && metadata.Title != ""

	// Toggle the toggle value on each update
	o.toggleValue = !o.toggleValue
	toggleInt := 0
	if o.toggleValue {
		toggleInt = 1
	}

	// Add DL Plus tags for artist and title if they exist and can be found
	o.addDLPlusTags(&content, formattedText, metadata)

	// Add DL_PLUS_ITEM_RUNNING (1 for tracks with artist+title, 0 for station/program info)
	runningInt := 0
	if isRunning {
		runningInt = 1
	}
	fmt.Fprintf(&content, "DL_PLUS_ITEM_RUNNING=%d\n", runningInt)

	// Add DL_PLUS_ITEM_TOGGLE (alternates 0/1 to indicate content changes)
	fmt.Fprintf(&content, "DL_PLUS_ITEM_TOGGLE=%d\n", toggleInt)

	// Write parameter block footer and display text
	content.WriteString("##### parameters } #####\n")
	content.WriteString(formattedText)

	return content.String()
}

// addDLPlusTags adds the DL_PLUS_TAG entries for artist and title
func (o *DLPlusOutput) addDLPlusTags(content *strings.Builder, formattedText string, metadata *core.Metadata) {
	// Add artist tag if artist exists and can be found in formatted text
	if metadata.Artist != "" {
		if bytePos := strings.Index(formattedText, metadata.Artist); bytePos >= 0 {
			// Convert byte position to rune position for DL Plus
			runePos := utf8.RuneCountInString(formattedText[:bytePos])
			length := utf8.RuneCountInString(metadata.Artist) - 1
			if length >= 0 {
				fmt.Fprintf(content, "DL_PLUS_TAG=%d %d %d\n", dlPlusTypeArtist, runePos, length)
			}
		}
	}

	// Add title tag if title exists and can be found in formatted text
	if metadata.Title != "" {
		if bytePos := strings.Index(formattedText, metadata.Title); bytePos >= 0 {
			// Convert byte position to rune position for DL Plus
			runePos := utf8.RuneCountInString(formattedText[:bytePos])
			length := utf8.RuneCountInString(metadata.Title) - 1
			if length >= 0 {
				fmt.Fprintf(content, "DL_PLUS_TAG=%d %d %d\n", dlPlusTypeTitle, runePos, length)
			}
		}
	}
}

// writeToFile writes the content to the configured file.
func (o *DLPlusOutput) writeToFile(content string) error {
	return utils.WriteFile(o.settings.Filename, []byte(content))
}

// Start initializes the output
func (o *DLPlusOutput) Start(_ context.Context) error {
	slog.Info("DL Plus output writing to file", "output", o.GetName(), "filename", o.settings.Filename)

	// Create initial empty file
	if err := utils.WriteFile(o.settings.Filename, []byte("")); err != nil {
		return fmt.Errorf("failed to create DL Plus file: %w", err)
	}

	return nil
}

// Stop cleans up the output
func (o *DLPlusOutput) Stop() error {
	slog.Debug("Stopped DL Plus output", "output", o.GetName(), "filename", o.settings.Filename)
	return nil
}
