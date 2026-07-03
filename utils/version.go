// Package utils provides utility functions for file operations, JSON processing,
// HTTP requests, version information, and WebSocket management.
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
	t, err := time.Parse(time.RFC3339, BuildTime)
	if err != nil {
		return time.Now().Format("2006")
	}
	return t.Format("2006")
}
