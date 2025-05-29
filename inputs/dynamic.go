package inputs

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
	"zwfm-metadata/config"
	"zwfm-metadata/core"
)

// DynamicInput handles dynamic HTTP input
type DynamicInput struct {
	*core.BaseInput
	core.WaitForShutdown
	settings config.DynamicInputSettings
}

// NewDynamicInput creates a new dynamic input
func NewDynamicInput(name string, settings config.DynamicInputSettings) *DynamicInput {
	return &DynamicInput{
		BaseInput: core.NewBaseInput(name),
		settings:  settings,
	}
}

// Start implements the Input interface

// UpdateMetadata updates the metadata from HTTP request
func (d *DynamicInput) UpdateMetadata(songID, artist, title, duration, secret string) error {
	// Check secret
	if d.settings.Secret != "" && secret != d.settings.Secret {
		return fmt.Errorf("invalid secret")
	}

	// Check required fields - only title is required
	if title == "" {
		return fmt.Errorf("title is required")
	}

	metadata := &core.Metadata{
		Name:      d.GetName(),
		SongID:    songID,
		Artist:    artist,
		Title:     title,
		Duration:  duration,
		UpdatedAt: time.Now(),
	}

	// Calculate expiration
	switch d.settings.Expiration.Type {
	case "dynamic":
		expiresAt := d.calculateDynamicExpiration(duration)
		metadata.ExpiresAt = &expiresAt
	case "fixed":
		expiresAt := time.Now().Add(time.Duration(d.settings.Expiration.Minutes) * time.Minute)
		metadata.ExpiresAt = &expiresAt
	case "none":
		// No expiration
	}

	d.SetMetadata(metadata)

	return nil
}

// calculateDynamicExpiration calculates expiration based on duration
func (d *DynamicInput) calculateDynamicExpiration(duration string) time.Time {
	// Parse duration (format: MM:SS or HH:MM:SS)
	parts := strings.Split(duration, ":")
	var totalSeconds int

	if len(parts) == 2 {
		// MM:SS
		minutes, _ := strconv.Atoi(parts[0])
		seconds, _ := strconv.Atoi(parts[1])
		totalSeconds = minutes*60 + seconds
	} else if len(parts) == 3 {
		// HH:MM:SS
		hours, _ := strconv.Atoi(parts[0])
		minutes, _ := strconv.Atoi(parts[1])
		seconds, _ := strconv.Atoi(parts[2])
		totalSeconds = hours*3600 + minutes*60 + seconds
	}

	// Round up to next minute
	minutes := int(math.Ceil(float64(totalSeconds) / 60.0))

	return time.Now().Add(time.Duration(minutes) * time.Minute)
}
