package utils

// Build information (set via ldflags during build)
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildTime = "unknown"
)

// UserAgent returns the User-Agent string for HTTP requests
func UserAgent() string {
	return "zwfm-metadata/" + Version
}
