// Package formatters provides text formatting capabilities for metadata,
// including case transformations and specialized formatting for radio systems.
package formatters

import (
	"fmt"

	"zwfm-metadata/core"
)

// FormatterFactory creates new Formatter instances for the registry.
type FormatterFactory func() core.Formatter

var formatterRegistry = map[string]FormatterFactory{}

// RegisterFormatter adds a formatter factory to the global registry.
func RegisterFormatter(name string, factory FormatterFactory) {
	formatterRegistry[name] = factory
}

// GetFormatter creates a formatter instance by name from the registry.
func GetFormatter(name string) (core.Formatter, error) {
	factory, exists := formatterRegistry[name]
	if !exists {
		return nil, fmt.Errorf("unknown formatter: %s", name)
	}
	return factory(), nil
}
