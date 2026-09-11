package outputs

import (
	"time"

	"zwfm-metadata/core"
)

// UniversalMetadata is the shared serialized output shape.
type UniversalMetadata struct {
	Type              string     `json:"type,omitzero"`
	FormattedMetadata string     `json:"formatted_metadata"`
	SongID            string     `json:"songID,omitzero"`
	Title             string     `json:"title"`
	Artist            string     `json:"artist,omitzero"`
	Duration          string     `json:"duration,omitzero"`
	UpdatedAt         time.Time  `json:"updated_at"`
	ExpiresAt         *time.Time `json:"expires_at,omitzero"`
	Source            string     `json:"source,omitzero"`
	SourceType        string     `json:"source_type,omitzero"`
}

// ConvertStructuredText preserves original timestamps and unformatted fields.
func ConvertStructuredText(st *core.StructuredText) *UniversalMetadata {
	if st == nil {
		return &UniversalMetadata{UpdatedAt: time.Now()}
	}

	um := &UniversalMetadata{
		FormattedMetadata: st.String(),
		Title:             st.Title,
		Artist:            st.Artist,
		Source:            st.InputName,
		SourceType:        st.InputType,
		UpdatedAt:         time.Now(),
	}

	if st.Original != nil {
		um.SongID = st.Original.SongID
		um.Duration = st.Original.Duration
		um.UpdatedAt = st.Original.UpdatedAt
		um.ExpiresAt = st.Original.ExpiresAt
	}

	return um
}

// ToTemplateData converts UniversalMetadata to template data for payload mapping.
// Every key is always present so templates never render "<no value>".
func (um *UniversalMetadata) ToTemplateData() map[string]any {
	expiresAt := ""
	if um.ExpiresAt != nil {
		expiresAt = um.ExpiresAt.Format(time.RFC3339)
	}

	return map[string]any{
		"type":               um.Type,
		"formatted_metadata": um.FormattedMetadata,
		"songID":             um.SongID,
		"title":              um.Title,
		"artist":             um.Artist,
		"duration":           um.Duration,
		"updated_at":         um.UpdatedAt.Format(time.RFC3339),
		"expires_at":         expiresAt,
		"source":             um.Source,
		"source_type":        um.SourceType,
	}
}
