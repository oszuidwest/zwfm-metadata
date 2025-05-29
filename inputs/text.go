package inputs

import (
	"time"
	"zwfm-metadata/config"
	"zwfm-metadata/core"
)

// TextInput handles static text input
type TextInput struct {
	*core.BaseInput
	core.WaitForShutdown
	settings config.TextInputSettings
}

// NewTextInput creates a new text input
func NewTextInput(name string, settings config.TextInputSettings) *TextInput {
	input := &TextInput{
		BaseInput: core.NewBaseInput(name),
		settings:  settings,
	}

	// Set initial metadata
	input.SetMetadata(&core.Metadata{
		Name:      name,
		Title:     settings.Text,
		UpdatedAt: time.Now(),
	})

	return input
}
