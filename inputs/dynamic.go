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

// NewDynamicInput initializes an HTTP API-driven input with the given settings.
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
	}

	d.SetMetadata(metadata)

	return nil
}

// calculateDynamicExpiration parses duration and returns expiration time.
// Accepts the following formats:
//   - Seconds: "272" or "272.5" or "272,5" (272 seconds, with optional decimal/comma separator)
//   - MM:SS: "3:00" or "03:00" (3 minutes)
//   - HH:MM:SS: "1:30:00" or "01:30:00" (1 hour 30 minutes)
//
// Leading zeros are optional. Invalid formats result in immediate expiration or fixed fallback if configured.
func (d *DynamicInput) calculateDynamicExpiration(duration string) time.Time {
	duration = strings.TrimSpace(duration)

	totalSeconds, ok := parseDurationToSeconds(duration)
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

// secondsFormatRe matches whole seconds or seconds with decimal places.
var secondsFormatRe = regexp.MustCompile(`^\d+(?:[.,]\d+)?$`)

// parseDurationToSeconds parses a duration string to total seconds.
func parseDurationToSeconds(duration string) (int, bool) {
	if secondsFormatRe.MatchString(duration) {
		return parseSecondsFormat(duration)
	}
	if strings.Contains(duration, ":") {
		return parseTimeFormat(duration)
	}
	return 0, false
}

// parseSecondsFormat parses numeric duration like "272" or "272.5".
func parseSecondsFormat(duration string) (int, bool) {
	fs, err := strconv.ParseFloat(strings.ReplaceAll(duration, ",", "."), 64)
	if err != nil {
		slog.Error("Error converting numerical value to Float duration", "duration", duration, "error", err)
		return 0, false
	}
	return int(math.Round(fs)), true
}

// parseTimeFormat parses MM:SS or HH:MM:SS format.
func parseTimeFormat(duration string) (int, bool) {
	parts := strings.Split(duration, ":")
	switch len(parts) {
	case 2:
		return parseMMSS(parts)
	case 3:
		return parseHHMMSS(parts)
	default:
		return 0, false
	}
}

// parseMMSS parses MM:SS format like "3:00" or "03:00".
func parseMMSS(parts []string) (int, bool) {
	minutes, errM := strconv.Atoi(parts[0])
	seconds, errS := strconv.Atoi(parts[1])
	if errM != nil || errS != nil || minutes < 0 || seconds < 0 || seconds >= 60 {
		slog.Error("Invalid MM:SS duration format", "parts", parts, "expected", "MM:SS (e.g., '03:00')")
		return 0, false
	}
	return minutes*60 + seconds, true
}

// parseHHMMSS parses HH:MM:SS format like "1:30:00" or "01:30:00".
func parseHHMMSS(parts []string) (int, bool) {
	hours, errH := strconv.Atoi(parts[0])
	minutes, errM := strconv.Atoi(parts[1])
	seconds, errS := strconv.Atoi(parts[2])
	if errH != nil || errM != nil || errS != nil || hours < 0 || minutes < 0 || minutes >= 60 || seconds < 0 || seconds >= 60 {
		slog.Error("Invalid HH:MM:SS duration format", "parts", parts, "expected", "HH:MM:SS (e.g., '01:30:00')")
		return 0, false
	}
	return hours*3600 + minutes*60 + seconds, true
}
