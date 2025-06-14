package outputs

import (
	"context"
	"fmt"
	"os"
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
	return &DLSPlusOutput{
		OutputBase: core.NewOutputBase(name),
		settings:   settings,
	}
}

// GetDelay returns the configured delay for this output
func (o *DLSPlusOutput) GetDelay() int {
	return o.settings.Delay
}

// SendFormattedMetadata is not used for DLS Plus output
func (o *DLSPlusOutput) SendFormattedMetadata(text string) {
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
		utils.LogError("Failed to write DLS Plus file %s: %v", o.settings.Filename, err)
		return
	}

	utils.LogDebug("Wrote DLS Plus to %s", o.settings.Filename)
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
				content.WriteString(fmt.Sprintf("DL_PLUS_TAG=%d %d %d\n", dlsPlusTypeArtist, pos, length))
			}
		}
	}

	// Add title tag if title exists and can be found in formatted text
	if metadata.Title != "" {
		if pos := strings.Index(formattedText, metadata.Title); pos >= 0 {
			length := len(metadata.Title) - 1
			if length >= 0 {
				content.WriteString(fmt.Sprintf("DL_PLUS_TAG=%d %d %d\n", dlsPlusTypeTitle, pos, length))
			}
		}
	}
}

// writeToFile writes the content to the configured file
func (o *DLSPlusOutput) writeToFile(content string) error {
	return os.WriteFile(o.settings.Filename, []byte(content), 0644)
}

// Start initializes the output
func (o *DLSPlusOutput) Start(ctx context.Context) error {
	utils.LogInfo("DLS Plus output writing to: %s", o.settings.Filename)

	// Create initial empty file
	if err := os.WriteFile(o.settings.Filename, []byte(""), 0644); err != nil {
		return fmt.Errorf("failed to create DLS Plus file: %w", err)
	}

	return nil
}

// Stop cleans up the output
func (o *DLSPlusOutput) Stop() error {
	utils.LogDebug("Stopped DLS Plus output: %s", o.settings.Filename)
	return nil
}
