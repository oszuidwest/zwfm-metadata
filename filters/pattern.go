package filters

import (
	"fmt"
	"regexp"
	"strings"

	"zwfm-metadata/config"
	"zwfm-metadata/core"
)

// Valid field values for PatternFilter.
const (
	FieldArtist = "artist"
	FieldTitle  = "title"
	FieldBoth   = "both"
)

// Valid action values for PatternFilter.
const (
	ActionClear = "clear"
	ActionSkip  = "skip"
)

// PatternFilter rejects or modifies metadata when fields match a regex pattern.
type PatternFilter struct {
	field   string
	pattern *regexp.Regexp
	action  string
}

// NewPatternFilter creates a filter that matches metadata fields against a regex pattern.
// field must be "artist", "title", or "both".
// action must be "clear" (default) to empty the matching field, or "skip" to reject entirely.
func NewPatternFilter(field, pattern, action string) (*PatternFilter, error) {
	field = strings.ToLower(field)
	if field != FieldArtist && field != FieldTitle && field != FieldBoth {
		return nil, fmt.Errorf("invalid field %q: must be %s, %s, or %s", field, FieldArtist, FieldTitle, FieldBoth)
	}

	action = strings.ToLower(action)
	if action == "" {
		action = ActionClear
	}
	if action != ActionClear && action != ActionSkip {
		return nil, fmt.Errorf("invalid action %q: must be %s or %s", action, ActionClear, ActionSkip)
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid pattern %q: %w", pattern, err)
	}

	return &PatternFilter{
		field:   field,
		pattern: re,
		action:  action,
	}, nil
}

func init() {
	RegisterFilter("pattern", func(cfg *config.FilterConfig) (core.Filter, error) {
		return NewPatternFilter(cfg.Field, cfg.Pattern, cfg.Action)
	})
}

// Decide checks if the metadata matches the pattern and returns the action to take.
func (p *PatternFilter) Decide(st *core.StructuredText) core.FilterResult {
	// Cache match results to avoid duplicate regex evaluation
	artistMatched := p.field != FieldTitle && p.pattern.MatchString(st.Artist)
	titleMatched := p.field != FieldArtist && p.pattern.MatchString(st.Title)

	if !artistMatched && !titleMatched {
		return core.FilterResult{Pass: true}
	}

	if p.action == ActionSkip {
		return core.FilterResult{Pass: false, ClearAll: true}
	}

	// action == ActionClear
	return core.FilterResult{
		Pass:        true,
		ClearArtist: artistMatched,
		ClearTitle:  titleMatched,
	}
}
