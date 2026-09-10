package inputs

import (
	"errors"
	"fmt"
	"log/slog"
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
func NewDynamicInput(name string, settings config.DynamicInputConfig) (*DynamicInput, error) {
	switch settings.Expiration.Type {
	case "dynamic", "fixed", "none", "":
	default:
		return nil, fmt.Errorf("expiration.type must be dynamic, fixed, or none, got %q", settings.Expiration.Type)
	}

	return &DynamicInput{
		InputBase: core.NewInputBase(name),
		settings:  settings,
	}, nil
}

// UpdateMetadata updates the metadata from an HTTP request.
func (d *DynamicInput) UpdateMetadata(update *core.MetadataRequest) error {
	if update == nil {
		return errors.New("metadata update is required")
	}

	if d.settings.Secret != "" && update.Secret != d.settings.Secret {
		return errors.New("invalid secret")
	}

	if update.Title == "" {
		return errors.New("title is required")
	}

	metadata := &core.Metadata{
		SongID:    update.SongID,
		Artist:    update.Artist,
		Title:     update.Title,
		Duration:  update.Duration,
		UpdatedAt: time.Now(),
	}

	switch d.settings.Expiration.Type {
	case "dynamic":
		metadata.ExpiresAt = new(d.dynamicExpiration(update.Duration))
	case "fixed":
		metadata.ExpiresAt = new(d.fixedExpiration())
	}

	d.SetMetadata(metadata)

	return nil
}

// fixedExpiration is now plus the configured minutes, which is "now" when none are configured.
func (d *DynamicInput) fixedExpiration() time.Time {
	return time.Now().Add(time.Duration(d.settings.Expiration.Minutes) * time.Minute)
}

// dynamicExpiration expires the track when its duration has elapsed. A missing or
// invalid duration falls back to the fixed expiration, which is immediate when no
// minutes are configured.
func (d *DynamicInput) dynamicExpiration(duration string) time.Time {
	seconds, ok := utils.ParseDurationToSeconds(duration)
	if ok && seconds > 0 {
		return time.Now().Add(time.Duration(seconds) * time.Second)
	}

	slog.Error("Invalid duration - using fixed expiration",
		"input", d.GetName(),
		"duration", duration,
		"expected", "positive seconds, MM:SS, or HH:MM:SS",
		"fallback_minutes", d.settings.Expiration.Minutes,
	)
	return d.fixedExpiration()
}
