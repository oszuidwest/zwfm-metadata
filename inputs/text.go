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
}

// NewTextInput creates a TextInput with pre-populated static metadata.
func NewTextInput(name string, settings config.TextInputConfig) *TextInput {
	input := &TextInput{
		InputBase: core.NewInputBase(name),
	}

	input.SetMetadata(&core.Metadata{
		Title:     settings.Text,
		UpdatedAt: time.Now(),
	})

	return input
}
