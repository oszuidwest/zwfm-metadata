package utils

import (
	"time"
	"zwfm-metadata/core"
)

// UniversalMetadata represents the common metadata structure used across all outputs
type UniversalMetadata struct {
	Type              string     `json:"type,omitempty" xml:"type,omitempty" yaml:"type,omitempty"`
	FormattedMetadata string     `json:"formatted_metadata" xml:"formatted_metadata" yaml:"formatted_metadata"`
	SongID            string     `json:"songID,omitempty" xml:"songID,omitempty" yaml:"songID,omitempty"`
	Title             string     `json:"title" xml:"title" yaml:"title"`
	Artist            string     `json:"artist,omitempty" xml:"artist,omitempty" yaml:"artist,omitempty"`
	Duration          string     `json:"duration,omitempty" xml:"duration,omitempty" yaml:"duration,omitempty"`
	UpdatedAt         time.Time  `json:"updated_at" xml:"updated_at" yaml:"updated_at"`
	ExpiresAt         *time.Time `json:"expires_at,omitempty" xml:"expires_at,omitempty" yaml:"expires_at,omitempty"`
	Source            string     `json:"source,omitempty" xml:"source,omitempty" yaml:"source,omitempty"`
	SourceType        string     `json:"source_type,omitempty" xml:"source_type,omitempty" yaml:"source_type,omitempty"`
}

// ConvertMetadata converts core.Metadata to UniversalMetadata
func ConvertMetadata(formattedText string, metadata *core.Metadata, source, sourceType string) *UniversalMetadata {
	return &UniversalMetadata{
		FormattedMetadata: formattedText,
		SongID:            metadata.SongID,
		Title:             metadata.Title,
		Artist:            metadata.Artist,
		Duration:          metadata.Duration,
		UpdatedAt:         metadata.UpdatedAt,
		ExpiresAt:         metadata.ExpiresAt,
		Source:            source,
		SourceType:        sourceType,
	}
}

// ConvertMetadataWithType converts core.Metadata to UniversalMetadata with a specific type
func ConvertMetadataWithType(formattedText string, metadata *core.Metadata, metadataType, source, sourceType string) *UniversalMetadata {
	universal := ConvertMetadata(formattedText, metadata, source, sourceType)
	universal.Type = metadataType
	return universal
}

// ToTemplateData converts UniversalMetadata to template data for payload mapping
func (um *UniversalMetadata) ToTemplateData() map[string]interface{} {
	data := map[string]interface{}{
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
