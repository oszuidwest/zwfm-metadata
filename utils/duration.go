package utils

import (
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

	if secondsFormatRe.MatchString(duration) {
		seconds, err := strconv.ParseFloat(strings.ReplaceAll(duration, ",", "."), 64)
		if err != nil {
			return 0, false
		}
		return int(math.Round(seconds)), true
	}

	// MM:SS or HH:MM:SS; every part after the first must be below 60.
	parts := strings.Split(duration, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return 0, false
	}

	total := 0
	for i, part := range parts {
		n, err := strconv.Atoi(part)
		if err != nil || n < 0 || (i > 0 && n >= 60) {
			return 0, false
		}
		total = total*60 + n
	}
	return total, true
}
