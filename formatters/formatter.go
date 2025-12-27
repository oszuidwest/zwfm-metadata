// Package formatters provides text formatting capabilities for metadata,
// including case transformations and specialized formatting for radio systems.
package formatters

import (
	"fmt"

	"zwfm-metadata/core"
)

// FormatterFactory creates new Formatter instances.
type FormatterFactory func() core.Formatter

var formatterRegistry = map[string]FormatterFactory{}

// RegisterFormatter registers a formatter factory under the given name.
func RegisterFormatter(name string, factory FormatterFactory) {
	formatterRegistry[name] = factory
}

// GetFormatter returns a new formatter instance for the given name.
func GetFormatter(name string) (core.Formatter, error) {
	factory, exists := formatterRegistry[name]
	if !exists {
		return nil, fmt.Errorf("unknown formatter: %s", name)
	}
	return factory(), nil
}
