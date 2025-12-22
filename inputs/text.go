// Package inputs provides various metadata input sources including static text,
// URL polling, and dynamic HTTP endpoint inputs for the metadata router.
package inputs

import (
	"time"
	"zwfm-metadata/config"
	"zwfm-metadata/core"
)

// TextInput provides static text metadata, typically used as a fallback source.
type TextInput struct {
	*core.InputBase
	core.PassiveComponent
	settings config.TextInputConfig
}

// NewTextInput creates a TextInput with pre-populated static metadata.
func NewTextInput(name string, settings config.TextInputConfig) *TextInput {
	input := &TextInput{
		InputBase: core.NewInputBase(name),
		settings:  settings,
	}

	input.SetMetadata(&core.Metadata{
		Name:      name,
		Title:     settings.Text,
		UpdatedAt: time.Now(),
	})

	return input
}
