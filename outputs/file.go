package outputs

import (
	"os"
	"zwfm-metadata/config"
	"zwfm-metadata/core"
	"zwfm-metadata/utils"
)

// FileOutput handles writing metadata to files
type FileOutput struct {
	*core.OutputBase
	core.PassiveComponent
	settings config.FileOutputConfig
}

// NewFileOutput creates a new file output
func NewFileOutput(name string, settings config.FileOutputConfig) *FileOutput {
	return &FileOutput{
		OutputBase: core.NewOutputBase(name),
		settings:   settings,
	}
}

// GetDelay implements the Output interface
func (f *FileOutput) GetDelay() int {
	return f.settings.Delay
}

// SendFormattedMetadata implements the Output interface (called by metadata router)
func (f *FileOutput) SendFormattedMetadata(formattedText string) {
	// Check if value changed to avoid unnecessary file writes
	if !f.HasChanged(formattedText) {
		return
	}

	// Write to file
	if err := f.writeToFile(formattedText); err != nil {
		utils.LogError("Failed to write metadata to file output %s: %v", f.GetName(), err)
	}
}

// writeToFile writes the metadata to the file
func (f *FileOutput) writeToFile(metadata string) error {
	// Write to file (overwrite)
	err := os.WriteFile(f.settings.Filename, []byte(metadata), 0644)
	if err != nil {
		return err
	}

	utils.LogDebug("Successfully wrote to file %s: %s", f.settings.Filename, metadata)

	return nil
}
