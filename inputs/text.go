package inputs

import (
	"time"
	"zwfm-metadata/config"
	"zwfm-metadata/core"
)

// TextInput handles static text input
type TextInput struct {
	*core.InputBase
	core.PassiveComponent
	settings config.TextInputConfig
}

// NewTextInput creates a new text input
func NewTextInput(name string, settings config.TextInputConfig) *TextInput {
	input := &TextInput{
		InputBase: core.NewInputBase(name),
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
