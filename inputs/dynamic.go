package inputs

import (
	"fmt"
	"log/slog"
	"math"
	"strconv"
	"strings"
	"time"
	"zwfm-metadata/config"
	"zwfm-metadata/core"
)

// DynamicInput handles dynamic HTTP input
type DynamicInput struct {
	*core.InputBase
	core.PassiveComponent
	settings config.DynamicInputConfig
}

// NewDynamicInput creates a new dynamic input
func NewDynamicInput(name string, settings config.DynamicInputConfig) *DynamicInput {
	return &DynamicInput{
		InputBase: core.NewInputBase(name),
		settings:  settings,
	}
}

// Start implements the Input interface

// UpdateMetadata updates the metadata from HTTP request
// Duration parameter accepts MM:SS or HH:MM:SS formats (leading zeros optional):
//   - MM:SS format: "3:00" or "03:00" (3 minutes)
//   - HH:MM:SS format: "1:30:00" or "01:30:00" (1 hour 30 minutes)
//
// Invalid formats will cause immediate expiration (no defaults)
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
// Accepts MM:SS (e.g., "3:00" or "03:00") or HH:MM:SS (e.g., "1:30:00" or "01:30:00") formats
// Leading zeros are optional. Invalid formats result in immediate expiration (no defaults)
func (d *DynamicInput) calculateDynamicExpiration(duration string) time.Time {
	var totalSeconds int

	// Parse duration - only accept MM:SS or HH:MM:SS formats
	parts := strings.Split(duration, ":")

	if len(parts) == 2 {
		// MM:SS format (e.g., "03:00")
		minutes, errM := strconv.Atoi(parts[0])
		seconds, errS := strconv.Atoi(parts[1])
		if errM == nil && errS == nil && minutes >= 0 && seconds >= 0 && seconds < 60 {
			totalSeconds = minutes*60 + seconds
		} else {
			slog.Error("Invalid MM:SS duration format - will expire immediately", "input", d.GetName(), "duration", duration, "expected", "MM:SS (e.g., '03:00')")
			return time.Now() // Immediate expiration
		}
	} else if len(parts) == 3 {
		// HH:MM:SS format (e.g., "01:30:00")
		hours, errH := strconv.Atoi(parts[0])
		minutes, errM := strconv.Atoi(parts[1])
		seconds, errS := strconv.Atoi(parts[2])
		if errH == nil && errM == nil && errS == nil && hours >= 0 && minutes >= 0 && minutes < 60 && seconds >= 0 && seconds < 60 {
			totalSeconds = hours*3600 + minutes*60 + seconds
		} else {
			slog.Error("Invalid HH:MM:SS duration format - will expire immediately", "input", d.GetName(), "duration", duration, "expected", "HH:MM:SS (e.g., '01:30:00')")
			return time.Now() // Immediate expiration
		}
	} else {
		slog.Error("Unsupported duration format - will expire immediately", "input", d.GetName(), "duration", duration, "expected", "MM:SS or HH:MM:SS format only")
		return time.Now() // Immediate expiration
	}

	// Validate minimum duration
	if totalSeconds <= 0 {
		slog.Error("Duration must be greater than 0 seconds - will expire immediately", "input", d.GetName(), "duration", duration)
		return time.Now() // Immediate expiration
	}

	// Round up to next minute
	minutes := int(math.Ceil(float64(totalSeconds) / 60.0))

	expiresAt := time.Now().Add(time.Duration(minutes) * time.Minute)
	slog.Debug("Calculated dynamic expiration", "input", d.GetName(), "duration", duration, "totalSeconds", totalSeconds, "roundedMinutes", minutes, "expiresAt", expiresAt.Format("15:04:05"))

	return expiresAt
}
