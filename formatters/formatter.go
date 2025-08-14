package formatters

import (
	"fmt"
)

// Formatter interface for text transformations
type Formatter interface {
	Format(text string) string
}

// FormatterFactory is a function type that creates a new formatter instance
type FormatterFactory func() Formatter

// formatterRegistry holds all registered formatters
var formatterRegistry = map[string]FormatterFactory{}

// RegisterFormatter registers a new formatter factory
func RegisterFormatter(name string, factory FormatterFactory) {
	formatterRegistry[name] = factory
}

// GetFormatter returns a formatter by name
func GetFormatter(name string) (Formatter, error) {
	factory, exists := formatterRegistry[name]
	if !exists {
		return nil, fmt.Errorf("unknown formatter: %s", name)
	}
	return factory(), nil
}
