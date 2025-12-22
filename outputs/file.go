// Package outputs provides various metadata output destinations including
// files, HTTP endpoints, WebSockets, and radio broadcasting systems.
package outputs

import (
	"log/slog"
	"zwfm-metadata/config"
	"zwfm-metadata/core"
	"zwfm-metadata/utils"
)

// FileOutput writes metadata to local files.
type FileOutput struct {
	*core.OutputBase
	core.PassiveComponent
	settings config.FileOutputConfig
}

// NewFileOutput creates a FileOutput with the given name and settings.
func NewFileOutput(name string, settings config.FileOutputConfig) *FileOutput {
	output := &FileOutput{
		OutputBase: core.NewOutputBase(name),
		settings:   settings,
	}
	output.SetDelay(settings.Delay)
	return output
}

// SendFormattedMetadata writes metadata to the configured file.
func (f *FileOutput) SendFormattedMetadata(formattedText string) {
	if !f.HasChanged(formattedText) {
		return
	}

	if err := f.writeToFile(formattedText); err != nil {
		slog.Error("Failed to write metadata to file", "output", f.GetName(), "error", err)
	}
}

func (f *FileOutput) writeToFile(metadata string) error {
	if err := utils.WriteFile(f.settings.Filename, []byte(metadata)); err != nil {
		return err
	}
	slog.Debug("Successfully wrote to file", "filename", f.settings.Filename, "metadata", metadata)
	return nil
}
