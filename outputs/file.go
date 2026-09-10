// Package outputs provides various metadata output destinations including
// files, HTTP endpoints, WebSockets, and radio broadcasting systems.
package outputs

import (
	"fmt"
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
	return &FileOutput{OutputBase: core.NewOutputBase(name), settings: settings}
}

// Send writes metadata to the configured file.
func (f *FileOutput) Send(st *core.StructuredText) error {
	content := st.String()
	if err := utils.WriteFile(f.settings.Filename, []byte(content)); err != nil {
		return fmt.Errorf("write metadata file: %w", err)
	}

	slog.Debug("Successfully wrote to file", "filename", f.settings.Filename, "metadata", content)
	return nil
}
