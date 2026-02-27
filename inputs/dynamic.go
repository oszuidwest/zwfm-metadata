package inputs

import (
	"fmt"
	"log/slog"
	"math"
	"time"
	"zwfm-metadata/config"
	"zwfm-metadata/core"
	"zwfm-metadata/utils"
)

// DynamicInput receives metadata via HTTP API calls with configurable expiration.
type DynamicInput struct {
	*core.InputBase
	core.PassiveComponent
	settings config.DynamicInputConfig
}

// NewDynamicInput initializes an HTTP API-driven input with the given settings.
func NewDynamicInput(name string, settings config.DynamicInputConfig) *DynamicInput {
	return &DynamicInput{
		InputBase: core.NewInputBase(name),
		settings:  settings,
	}
}

// UpdateMetadata updates the metadata from an HTTP request.
func (d *DynamicInput) UpdateMetadata(songID, artist, title, duration, secret string) error {
	if d.settings.Secret != "" && secret != d.settings.Secret {
		return fmt.Errorf("invalid secret")
	}

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

	switch d.settings.Expiration.Type {
	case "dynamic":
		expiresAt := d.calculateDynamicExpiration(duration)
		metadata.ExpiresAt = &expiresAt
	case "fixed":
		expiresAt := time.Now().Add(time.Duration(d.settings.Expiration.Minutes) * time.Minute)
		metadata.ExpiresAt = &expiresAt
	}

	d.SetMetadata(metadata)

	return nil
}

// calculateDynamicExpiration parses duration and returns expiration time.
func (d *DynamicInput) calculateDynamicExpiration(duration string) time.Time {
	totalSeconds, ok := utils.ParseDurationToSeconds(duration)
	if !ok {
		return d.handleUnsupportedFormat(duration)
	}

	if totalSeconds <= 0 {
		slog.Error("Duration must be greater than 0 seconds - will expire immediately", "input", d.GetName(), "duration", duration)
		return time.Now()
	}

	minutes := int(math.Ceil(float64(totalSeconds) / 60.0))
	expiresAt := time.Now().Add(time.Duration(minutes) * time.Minute)
	slog.Debug("Calculated dynamic expiration", "input", d.GetName(), "duration", duration, "totalSeconds", totalSeconds, "roundedMinutes", minutes, "expiresAt", expiresAt.Format("15:04:05"))

	return expiresAt
}

// handleUnsupportedFormat returns fallback expiration or immediate expiration.
func (d *DynamicInput) handleUnsupportedFormat(duration string) time.Time {
	if d.settings.Expiration.Minutes > 0 {
		expiresAt := time.Now().Add(time.Duration(d.settings.Expiration.Minutes) * time.Minute)
		slog.Error("Unsupported duration format - using fixed expiration", "input", d.GetName(), "duration", duration, "expected", "seconds, MM:SS, or HH:MM:SS format only")
		return expiresAt
	}
	slog.Error("Unsupported duration format - will expire immediately", "input", d.GetName(), "duration", duration, "expected", "seconds, MM:SS, or HH:MM:SS format only")
	return time.Now()
}
