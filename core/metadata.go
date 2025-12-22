package core

import (
	"strings"
	"time"
)

// IsExpired reports whether the metadata has expired.
func (m *Metadata) IsExpired() bool {
	if m == nil || m.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*m.ExpiresAt)
}

// IsAvailable reports whether metadata has meaningful content and is not expired.
func (m *Metadata) IsAvailable() bool {
	return m != nil && m.Title != "" && !m.IsExpired()
}

// Clone duplicates the metadata including a copy of the expiration time.
func (m *Metadata) Clone() *Metadata {
	if m == nil {
		return nil
	}

	clone := &Metadata{
		Name:      strings.Clone(m.Name),
		SongID:    strings.Clone(m.SongID),
		Artist:    strings.Clone(m.Artist),
		Title:     strings.Clone(m.Title),
		Duration:  strings.Clone(m.Duration),
		UpdatedAt: m.UpdatedAt,
	}

	if m.ExpiresAt != nil {
		expiresAt := *m.ExpiresAt
		clone.ExpiresAt = &expiresAt
	}

	return clone
}

// FormatString returns "Artist - Title" or just Title, empty if no content.
func (m *Metadata) FormatString() string {
	if m == nil || m.Title == "" {
		return ""
	}

	if m.Artist != "" && m.Title != "" {
		return m.Artist + " - " + m.Title
	}

	return m.Title
}
