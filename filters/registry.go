// Package filters provides metadata filtering capabilities that determine
// whether metadata should proceed to outputs based on configurable rules.
package filters

import (
	"fmt"

	"zwfm-metadata/config"
	"zwfm-metadata/core"
)

// FilterFactory creates a filter instance from a config.
type FilterFactory func(cfg *config.FilterConfig) (core.Filter, error)

var filterRegistry = map[string]FilterFactory{}

// RegisterFilter registers a filter factory under the given name.
func RegisterFilter(name string, factory FilterFactory) {
	filterRegistry[name] = factory
}

// GetFilter returns a new filter instance for the given config type.
func GetFilter(cfg *config.FilterConfig) (core.Filter, error) {
	if cfg == nil {
		return nil, fmt.Errorf("filter config is nil")
	}
	factory, exists := filterRegistry[cfg.Type]
	if !exists {
		return nil, fmt.Errorf("unknown filter: %s", cfg.Type)
	}
	return factory(cfg)
}
