package inputs

import (
	"fmt"
	"log/slog"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
	"zwfm-metadata/config"
	"zwfm-metadata/core"
)

// DynamicInput receives metadata via HTTP API calls with configurable expiration.
type DynamicInput struct {
	*core.InputBase
	core.PassiveComponent
	settings config.DynamicInputConfig
}

// NewDynamicInput creates a DynamicInput with the given name and settings.
func NewDynamicInput(name string, settings config.DynamicInputConfig) *DynamicInput {
	return &DynamicInput{
		InputBase: core.NewInputBase(name),
		settings:  settings,
	}
}

// UpdateMetadata updates the metadata from HTTP request.
// Duration parameter accepts the following formats (leading zeros optional):
//   - Seconds: "272" or "272.5" or "272,5" (272 seconds)
//   - MM:SS format: "3:00" or "03:00" (3 minutes)
//   - HH:MM:SS format: "1:30:00" or "01:30:00" (1 hour 30 minutes)
//
// Invalid formats will cause immediate expiration or fixed fallback if configured.
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
	case "none":
		// No expiration
	}

	d.SetMetadata(metadata)

	return nil
}

// calculateDynamicExpiration calculates expiration based on duration.
// Accepts the following formats:
//   - Seconds: "272" or "272.5" or "272,5" (272 seconds, with optional decimal/comma separator)
//   - MM:SS: "3:00" or "03:00" (3 minutes)
//   - HH:MM:SS: "1:30:00" or "01:30:00" (1 hour 30 minutes)
//
// Leading zeros are optional. Invalid formats result in immediate expiration or fixed fallback if configured.
func (d *DynamicInput) calculateDynamicExpiration(duration string) time.Time {
	var totalSeconds int

	// Accepts whole seconds or seconds with decimal places,
	// e.g. '272' or '272.5' or '272,670041666667'
	duration = strings.TrimSpace(duration)
	var secondsFormatRe = regexp.MustCompile(`^\d+(?:[.,]\d+)?$`)

	if secondsFormatRe.MatchString(duration) {
		fs, err := strconv.ParseFloat(strings.ReplaceAll(duration, ",", "."), 64)
		if err != nil {
			slog.Error("Error converting numerical value to Float duration", "input", d.GetName(), "duration", duration, "error", err)
			return time.Now() // Immediate expiration
		}
		totalSeconds = int(math.Round(fs))
	} else if strings.Contains(duration, ":") {
		// Time format (MM:SS or HH:MM:SS)
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
			// Invalid time format
			if d.settings.Expiration.Minutes > 0 {
				expiresAt := time.Now().Add(time.Duration(d.settings.Expiration.Minutes) * time.Minute)
				slog.Error("Unsupported duration format - using fixed expiration", "input", d.GetName(), "duration", duration, "expected", "seconds, MM:SS, or HH:MM:SS format only")
				return expiresAt
			}
			slog.Error("Unsupported duration format - will expire immediately", "input", d.GetName(), "duration", duration, "expected", "seconds, MM:SS, or HH:MM:SS format only")
			return time.Now() // Immediate expiration
		}
	} else {
		// No recognized format at all - check for fixed expiration fallback
		if d.settings.Expiration.Minutes > 0 {
			expiresAt := time.Now().Add(time.Duration(d.settings.Expiration.Minutes) * time.Minute)
			slog.Error("Unsupported duration format - using fixed expiration", "input", d.GetName(), "duration", duration, "expected", "seconds, MM:SS, or HH:MM:SS format only")
			return expiresAt
		}
		slog.Error("Unsupported duration format - will expire immediately", "input", d.GetName(), "duration", duration, "expected", "seconds, MM:SS, or HH:MM:SS format only")
		return time.Now() // Immediate expiration
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
