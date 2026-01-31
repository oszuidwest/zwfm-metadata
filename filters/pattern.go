package filters

import (
	"fmt"
	"regexp"
	"strings"

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

// Filter checks if the metadata matches the pattern.
// Returns false to reject the metadata entirely (action="skip").
// Returns true to continue processing, possibly with cleared fields (action="clear").
func (p *PatternFilter) Filter(st *core.StructuredText) bool {
	var matched bool

	switch p.field {
	case FieldArtist:
		matched = p.pattern.MatchString(st.Artist)
	case FieldTitle:
		matched = p.pattern.MatchString(st.Title)
	case FieldBoth:
		matched = p.pattern.MatchString(st.Artist) || p.pattern.MatchString(st.Title)
	}

	if !matched {
		return true
	}

	if p.action == ActionSkip {
		st.Artist = ""
		st.Title = ""
		st.Prefix = ""
		st.Suffix = ""
		return false
	}

	switch p.field {
	case FieldArtist:
		st.Artist = ""
	case FieldTitle:
		st.Title = ""
	case FieldBoth:
		if p.pattern.MatchString(st.Artist) {
			st.Artist = ""
		}
		if p.pattern.MatchString(st.Title) {
			st.Title = ""
		}
	}

	return true
}
