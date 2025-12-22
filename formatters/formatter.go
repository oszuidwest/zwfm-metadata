// Package formatters provides text formatting capabilities for metadata strings,
// including various case transformations and specialized formatting for radio systems.
package formatters

import (
	"fmt"
)

// Formatter transforms metadata text for output-specific requirements.
type Formatter interface {
	Format(text string) string
}

// FormatterFactory creates new Formatter instances for the registry.
type FormatterFactory func() Formatter

var formatterRegistry = map[string]FormatterFactory{}

// RegisterFormatter adds a formatter factory to the global registry.
func RegisterFormatter(name string, factory FormatterFactory) {
	formatterRegistry[name] = factory
}

// GetFormatter creates a formatter instance by name from the registry.
func GetFormatter(name string) (Formatter, error) {
	factory, exists := formatterRegistry[name]
	if !exists {
		return nil, fmt.Errorf("unknown formatter: %s", name)
	}
	return factory(), nil
}
