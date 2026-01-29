// Package utils provides utility functions for file operations, JSON processing,
// payload mapping, version information, and WebSocket management.
package utils

import (
	"time"
)

// Build information (set via ldflags during build).
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildTime = "unknown"
)

// UserAgent returns the User-Agent string for HTTP requests.
func UserAgent() string {
	return "zwfm-metadata/" + Version
}

// GetBuildYear returns the year from the build time.
func GetBuildYear() string {
	if BuildTime == "unknown" {
		return time.Now().Format("2006")
	}
	t, err := time.Parse(time.RFC3339, BuildTime)
	if err != nil {
		t, err = time.Parse("2006-01-02T15:04:05Z", BuildTime)
		if err != nil {
			return time.Now().Format("2006")
		}
	}
	return t.Format("2006")
}
