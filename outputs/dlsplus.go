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

// Content type constants for DLS Plus tags
const (
	dlsPlusTypeTitle  = 1
	dlsPlusTypeArtist = 4
)

// DLSPlusOutput writes metadata in DLS Plus format for ODR-PadEnc
// It implements EnhancedOutput to access raw metadata fields
type DLSPlusOutput struct {
	*core.OutputBase
	core.PassiveComponent
	settings config.DLSPlusOutputConfig
}

// NewDLSPlusOutput creates a new DLS Plus output instance
func NewDLSPlusOutput(name string, settings config.DLSPlusOutputConfig) *DLSPlusOutput {
	output := &DLSPlusOutput{
		OutputBase: core.NewOutputBase(name),
		settings:   settings,
	}
	output.SetDelay(settings.Delay)
	return output
}

// SendFormattedMetadata is not used for DLS Plus output
func (o *DLSPlusOutput) SendFormattedMetadata(_ string) {
	// This won't be called since we implement EnhancedOutput
}

// SendEnhancedMetadata writes the metadata in DLS Plus format
func (o *DLSPlusOutput) SendEnhancedMetadata(formattedText string, metadata *core.Metadata) {
	// Check if value changed to avoid unnecessary file writes
	if !o.HasChanged(formattedText) {
		return
	}

	// Build the DLS Plus content
	content := o.buildDLSPlusContent(formattedText, metadata)

	// Write to file
	if err := o.writeToFile(content); err != nil {
		slog.Error("Failed to write DLS Plus file", "filename", o.settings.Filename, "error", err)
		return
	}

	slog.Debug("Wrote DLS Plus", "filename", o.settings.Filename)
}

// buildDLSPlusContent creates the DLS Plus formatted content
func (o *DLSPlusOutput) buildDLSPlusContent(formattedText string, metadata *core.Metadata) string {
	var content strings.Builder

	// Write parameter block header
	content.WriteString("##### parameters { #####\n")
	content.WriteString("DL_PLUS=1\n")

	// Add DLS Plus tags for artist and title if they exist and can be found
	o.addDLSPlusTags(&content, formattedText, metadata)

	// Write parameter block footer and display text
	content.WriteString("##### parameters } #####\n")
	content.WriteString(formattedText)

	return content.String()
}

// addDLSPlusTags adds the DL_PLUS_TAG entries for artist and title
func (o *DLSPlusOutput) addDLSPlusTags(content *strings.Builder, formattedText string, metadata *core.Metadata) {
	// Add artist tag if artist exists and can be found in formatted text
	if metadata.Artist != "" {
		if pos := strings.Index(formattedText, metadata.Artist); pos >= 0 {
			length := len(metadata.Artist) - 1
			if length >= 0 {
				fmt.Fprintf(content, "DL_PLUS_TAG=%d %d %d\n", dlsPlusTypeArtist, pos, length)
			}
		}
	}

	// Add title tag if title exists and can be found in formatted text
	if metadata.Title != "" {
		if pos := strings.Index(formattedText, metadata.Title); pos >= 0 {
			length := len(metadata.Title) - 1
			if length >= 0 {
				fmt.Fprintf(content, "DL_PLUS_TAG=%d %d %d\n", dlsPlusTypeTitle, pos, length)
			}
		}
	}
}

// writeToFile writes the content to the configured file
func (o *DLSPlusOutput) writeToFile(content string) error {
	return utils.WriteFile(o.settings.Filename, []byte(content))
}

// Start initializes the output
func (o *DLSPlusOutput) Start(_ context.Context) error {
	slog.Info("DLS Plus output writing to file", "filename", o.settings.Filename)

	// Create initial empty file
	if err := utils.WriteFile(o.settings.Filename, []byte("")); err != nil {
		return fmt.Errorf("failed to create DLS Plus file: %w", err)
	}

	return nil
}

// Stop cleans up the output
func (o *DLSPlusOutput) Stop() error {
	slog.Debug("Stopped DLS Plus output", "filename", o.settings.Filename)
	return nil
}
