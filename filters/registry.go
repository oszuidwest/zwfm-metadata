// Package filters selects metadata before output formatting.
package filters

import (
	"fmt"

	"zwfm-metadata/config"
	"zwfm-metadata/core"
)

// New creates the filter described by cfg.
func New(cfg *config.FilterConfig) (core.Filter, error) {
	switch cfg.Type {
	case "duration":
		return NewDurationFilter(cfg.MinSeconds)
	case "pattern":
		return NewPatternFilter(cfg.Field, cfg.Pattern, cfg.Action)
	default:
		return nil, fmt.Errorf("unknown filter: %s", cfg.Type)
	}
}
