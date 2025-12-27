package core

import (
	"strings"
	"unicode/utf8"
)

// StructuredText preserves field boundaries during formatting, enabling accurate
// position tracking for protocols like DAB+ Dynamic Label Plus.
type StructuredText struct {
	Original  *Metadata
	Prefix    string
	Artist    string
	Separator string
	Title     string
	Suffix    string
	InputName string
	InputType string
}

// NewStructuredText initializes a StructuredText with artist and title from the given metadata.
func NewStructuredText(m *Metadata) *StructuredText {
	if m == nil {
		return &StructuredText{Separator: " - "}
	}

	return &StructuredText{
		Original:  m,
		Artist:    m.Artist,
		Separator: " - ",
		Title:     m.Title,
	}
}

// String assembles all fields into a single display string.
func (st *StructuredText) String() string {
	if st == nil {
		return ""
	}

	var b strings.Builder
	b.Grow(len(st.Prefix) + len(st.Artist) + len(st.Separator) + len(st.Title) + len(st.Suffix))

	b.WriteString(st.Prefix)

	if st.Artist != "" {
		b.WriteString(st.Artist)
		if st.Title != "" {
			b.WriteString(st.Separator)
		}
	}

	b.WriteString(st.Title)
	b.WriteString(st.Suffix)

	return b.String()
}

// Len returns the rune count of the combined text for length-limited outputs.
func (st *StructuredText) Len() int {
	if st == nil {
		return 0
	}
	return utf8.RuneCountInString(st.String())
}

// ArtistRange reports whether the artist field is present, and if so,
// returns its start position and DL Plus-format length (actual_length - 1).
func (st *StructuredText) ArtistRange() (start, length int, ok bool) {
	if st == nil || st.Artist == "" {
		return 0, 0, false
	}

	start = utf8.RuneCountInString(st.Prefix)
	runeLen := utf8.RuneCountInString(st.Artist)

	return start, runeLen - 1, true
}

// TitleRange reports whether the title field is present, and if so,
// returns its start position and DL Plus-format length (actual_length - 1).
func (st *StructuredText) TitleRange() (start, length int, ok bool) {
	if st == nil || st.Title == "" {
		return 0, 0, false
	}

	start = utf8.RuneCountInString(st.Prefix)
	if st.Artist != "" {
		start += utf8.RuneCountInString(st.Artist)
		start += utf8.RuneCountInString(st.Separator)
	}

	runeLen := utf8.RuneCountInString(st.Title)

	return start, runeLen - 1, true
}

// HasContent reports whether artist or title contains text.
func (st *StructuredText) HasContent() bool {
	return st != nil && (st.Artist != "" || st.Title != "")
}

// IsRunning reports whether both artist and title are present for DL Plus ITEM_RUNNING.
func (st *StructuredText) IsRunning() bool {
	return st != nil && st.Artist != "" && st.Title != ""
}

// Clone duplicates the StructuredText with all its fields.
func (st *StructuredText) Clone() *StructuredText {
	if st == nil {
		return nil
	}

	return &StructuredText{
		Original:  st.Original,
		Prefix:    st.Prefix,
		Artist:    st.Artist,
		Separator: st.Separator,
		Title:     st.Title,
		Suffix:    st.Suffix,
		InputName: st.InputName,
		InputType: st.InputType,
	}
}
