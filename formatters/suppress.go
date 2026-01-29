package formatters

import (
	"log/slog"
	"regexp"

	"zwfm-metadata/core"
)

// SuppressFormatter clears metadata fields when they match a regex pattern.
type SuppressFormatter struct {
	field   string
	pattern *regexp.Regexp
	skip    bool // if true, marks metadata to be skipped entirely rather than clearing fields
}

// NewSuppressFormatter creates a formatter that suppresses metadata based on regex matching.
// field can be "artist", "title", or "both".
// action can be "clear" (default) to empty the matching field, or "skip" to suppress the entire update.
func NewSuppressFormatter(field, pattern, action string) (*SuppressFormatter, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}

	return &SuppressFormatter{
		field:   field,
		pattern: re,
		skip:    action == "skip",
	}, nil
}

// Format checks if the metadata matches the suppression pattern and clears or marks for skip.
func (s *SuppressFormatter) Format(st *core.StructuredText) {
	var matched bool
	var matchedField, matchedValue string

	switch s.field {
	case "artist":
		if s.pattern.MatchString(st.Artist) {
			matched = true
			matchedField = "artist"
			matchedValue = st.Artist
		}
	case "title":
		if s.pattern.MatchString(st.Title) {
			matched = true
			matchedField = "title"
			matchedValue = st.Title
		}
	case "both":
		if s.pattern.MatchString(st.Artist) {
			matched = true
			matchedField = "artist"
			matchedValue = st.Artist
		} else if s.pattern.MatchString(st.Title) {
			matched = true
			matchedField = "title"
			matchedValue = st.Title
		}
	}

	if !matched {
		return
	}

	slog.Debug("Suppress filter matched",
		"field", matchedField,
		"value", matchedValue,
		"pattern", s.pattern.String(),
		"action", map[bool]string{true: "skip", false: "clear"}[s.skip])

	if s.skip {
		// Clear all fields to suppress the entire update
		st.Artist = ""
		st.Title = ""
		st.Prefix = ""
		st.Suffix = ""
	} else {
		// Clear only the matching field(s)
		switch s.field {
		case "artist":
			st.Artist = ""
		case "title":
			st.Title = ""
		case "both":
			if s.pattern.MatchString(st.Artist) {
				st.Artist = ""
			}
			if s.pattern.MatchString(st.Title) {
				st.Title = ""
			}
		}
	}
}
