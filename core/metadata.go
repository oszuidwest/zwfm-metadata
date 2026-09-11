package core

import "time"

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
