package utils

import (
	"time"

	"zwfm-metadata/core"
)

// UniversalMetadata represents the common metadata structure used across all outputs.
type UniversalMetadata struct {
	Type              string     `json:"type,omitzero" xml:"type,omitempty"`
	FormattedMetadata string     `json:"formatted_metadata" xml:"formatted_metadata"`
	SongID            string     `json:"songID,omitzero" xml:"songID,omitempty"`
	Title             string     `json:"title" xml:"title"`
	Artist            string     `json:"artist,omitzero" xml:"artist,omitempty"`
	Duration          string     `json:"duration,omitzero" xml:"duration,omitempty"`
	UpdatedAt         time.Time  `json:"updated_at" xml:"updated_at"`
	ExpiresAt         *time.Time `json:"expires_at,omitzero" xml:"expires_at,omitempty"`
	Source            string     `json:"source,omitzero" xml:"source,omitempty"`
	SourceType        string     `json:"source_type,omitzero" xml:"source_type,omitempty"`
}

// ConvertStructuredText converts a StructuredText to UniversalMetadata.
func ConvertStructuredText(st *core.StructuredText) *UniversalMetadata {
	if st == nil {
		return &UniversalMetadata{
			UpdatedAt: time.Now(),
		}
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

// ConvertStructuredTextWithType converts StructuredText with a specific type.
func ConvertStructuredTextWithType(st *core.StructuredText, metadataType string) *UniversalMetadata {
	um := ConvertStructuredText(st)
	um.Type = metadataType
	return um
}

// ToTemplateData converts UniversalMetadata to template data for payload mapping.
func (um *UniversalMetadata) ToTemplateData() map[string]any {
	data := map[string]any{
		"formatted_metadata": um.FormattedMetadata,
		"songID":             um.SongID,
		"title":              um.Title,
		"artist":             um.Artist,
		"duration":           um.Duration,
		"updated_at":         um.UpdatedAt.Format(time.RFC3339),
	}

	if um.Type != "" {
		data["type"] = um.Type
	}

	if um.Source != "" {
		data["source"] = um.Source
	}

	if um.SourceType != "" {
		data["source_type"] = um.SourceType
	}

	if um.ExpiresAt != nil {
		data["expires_at"] = um.ExpiresAt.Format(time.RFC3339)
	} else {
		data["expires_at"] = ""
	}

	return data
}
