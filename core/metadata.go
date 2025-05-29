package core

import (
	"time"
)

// IsExpired checks if the metadata has expired
func (m *Metadata) IsExpired() bool {
	if m == nil || m.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*m.ExpiresAt)
}

// IsAvailable checks if metadata has meaningful content and is not expired
func (m *Metadata) IsAvailable() bool {
	return m != nil && m.Title != "" && !m.IsExpired()
}

// Clone creates a deep copy of the metadata
func (m *Metadata) Clone() *Metadata {
	if m == nil {
		return nil
	}

	clone := &Metadata{
		Name:      m.Name,
		SongID:    m.SongID,
		Artist:    m.Artist,
		Title:     m.Title,
		Duration:  m.Duration,
		UpdatedAt: m.UpdatedAt,
	}

	if m.ExpiresAt != nil {
		expiresAt := *m.ExpiresAt
		clone.ExpiresAt = &expiresAt
	}

	return clone
}

// FormatString returns a formatted string representation
func (m *Metadata) FormatString() string {
	if m == nil || m.Title == "" {
		return "" // Return empty if no title - makes input "unavailable"
	}

	if m.Artist != "" && m.Title != "" {
		return m.Artist + " - " + m.Title
	}

	return m.Title
}
