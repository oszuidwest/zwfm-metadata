package utils

import (
	"log/slog"
	"math"
	"regexp"
	"strconv"
	"strings"
)

// secondsFormatRe matches whole seconds or seconds with decimal places.
var secondsFormatRe = regexp.MustCompile(`^\d+(?:[.,]\d+)?$`)

// ParseDurationToSeconds parses a duration string to total seconds.
// Supports formats: "272", "272.5", "3:45", "03:45", "1:30:00".
func ParseDurationToSeconds(duration string) (int, bool) {
	duration = strings.TrimSpace(duration)
	if duration == "" {
		return 0, false
	}

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
		slog.Debug("Failed to parse seconds format", "duration", duration, "error", err)
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
		slog.Debug("Failed to parse MM:SS format", "parts", parts, "errM", errM, "errS", errS)
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
		slog.Debug("Failed to parse HH:MM:SS format", "parts", parts, "errH", errH, "errM", errM, "errS", errS)
		return 0, false
	}
	return hours*3600 + minutes*60 + seconds, true
}
