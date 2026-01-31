package filters

import (
	"fmt"
	"regexp"
	"strings"

	"zwfm-metadata/core"
)

// Valid field values for SuppressFilter.
const (
	FieldArtist = "artist"
	FieldTitle  = "title"
	FieldBoth   = "both"
)

// Valid action values for SuppressFilter.
const (
	ActionClear = "clear"
	ActionSkip  = "skip"
)

// SuppressFilter rejects or modifies metadata when fields match a regex pattern.
type SuppressFilter struct {
	field   string
	pattern *regexp.Regexp
	action  string
}

// NewSuppressFilter creates a filter that suppresses metadata based on regex matching.
// field must be "artist", "title", or "both".
// action must be "clear" (default) to empty the matching field, or "skip" to reject entirely.
func NewSuppressFilter(field, pattern, action string) (*SuppressFilter, error) {
	// Validate field
	field = strings.ToLower(field)
	if field != FieldArtist && field != FieldTitle && field != FieldBoth {
		return nil, fmt.Errorf("invalid field %q: must be %s, %s, or %s", field, FieldArtist, FieldTitle, FieldBoth)
	}

	// Validate and normalize action
	action = strings.ToLower(action)
	if action == "" {
		action = ActionClear
	}
	if action != ActionClear && action != ActionSkip {
		return nil, fmt.Errorf("invalid action %q: must be %s or %s", action, ActionClear, ActionSkip)
	}

	// Compile regex pattern
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid pattern %q: %w", pattern, err)
	}

	return &SuppressFilter{
		field:   field,
		pattern: re,
		action:  action,
	}, nil
}

// Filter checks if the metadata matches the suppression pattern.
// Returns false to reject the metadata entirely (action="skip").
// Returns true to continue processing, possibly with cleared fields (action="clear").
func (s *SuppressFilter) Filter(st *core.StructuredText) bool {
	var matched bool

	switch s.field {
	case FieldArtist:
		matched = s.pattern.MatchString(st.Artist)
	case FieldTitle:
		matched = s.pattern.MatchString(st.Title)
	case FieldBoth:
		matched = s.pattern.MatchString(st.Artist) || s.pattern.MatchString(st.Title)
	}

	if !matched {
		return true // No match, continue processing
	}

	if s.action == ActionSkip {
		// Reject entire update - clear all fields
		st.Artist = ""
		st.Title = ""
		st.Prefix = ""
		st.Suffix = ""
		return false // Reject
	}

	// Action is "clear" - clear only the matching field(s), continue processing
	switch s.field {
	case FieldArtist:
		st.Artist = ""
	case FieldTitle:
		st.Title = ""
	case FieldBoth:
		if s.pattern.MatchString(st.Artist) {
			st.Artist = ""
		}
		if s.pattern.MatchString(st.Title) {
			st.Title = ""
		}
	}

	return true // Continue processing with cleared fields
}
