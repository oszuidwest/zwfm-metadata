package utils

import (
	"math"
	"regexp"
	"strconv"
	"strings"
)

var secondsFormatRe = regexp.MustCompile(`^\d+(?:[.,]\d+)?$`)

// ParseDurationToSeconds accepts seconds, MM:SS, or HH:MM:SS and rounds fractions.
func ParseDurationToSeconds(duration string) (int, bool) {
	duration = strings.TrimSpace(duration)

	if secondsFormatRe.MatchString(duration) {
		seconds, err := strconv.ParseFloat(strings.ReplaceAll(duration, ",", "."), 64)
		if err != nil {
			return 0, false
		}
		rounded := math.Round(seconds)
		if rounded >= float64(math.MaxInt) {
			return 0, false
		}
		return int(rounded), true
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
		if total > (math.MaxInt-n)/60 {
			return 0, false
		}
		total = total*60 + n
	}
	return total, true
}
