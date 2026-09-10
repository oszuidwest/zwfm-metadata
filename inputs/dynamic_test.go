package inputs

import (
	"strings"
	"testing"
	"time"

	"zwfm-metadata/config"
	"zwfm-metadata/core"
)

func TestDynamicInputUpdateMetadataExpiration(t *testing.T) {
	tests := []struct {
		name       string
		mode       string
		minutes    int
		duration   string
		wantExpiry bool
		wantAfter  time.Duration
	}{
		{name: "none", mode: "none"},
		{name: "fixed", mode: "fixed", minutes: 2, wantExpiry: true, wantAfter: 2 * time.Minute},
		{name: "dynamic", mode: "dynamic", duration: "90", wantExpiry: true, wantAfter: 90 * time.Second},
		{name: "dynamic zero", mode: "dynamic", duration: "0", wantExpiry: true},
		{name: "dynamic fallback", mode: "dynamic", minutes: 3, duration: "invalid", wantExpiry: true, wantAfter: 3 * time.Minute},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var settings config.DynamicInputConfig
			settings.Expiration.Type = tt.mode
			settings.Expiration.Minutes = tt.minutes
			input, err := NewDynamicInput("test", settings)
			if err != nil {
				t.Fatal(err)
			}

			before := time.Now()
			if err := input.UpdateMetadata(&core.MetadataRequest{Title: "Title", Duration: tt.duration}); err != nil {
				t.Fatal(err)
			}
			after := time.Now()
			expiresAt := input.GetMetadata().ExpiresAt
			if !tt.wantExpiry {
				if expiresAt != nil {
					t.Fatalf("ExpiresAt = %v, want nil", expiresAt)
				}
				return
			}
			if expiresAt == nil || expiresAt.Before(before.Add(tt.wantAfter)) || expiresAt.After(after.Add(tt.wantAfter)) {
				t.Fatalf("ExpiresAt = %v, want between %v and %v", expiresAt, before.Add(tt.wantAfter), after.Add(tt.wantAfter))
			}
		})
	}
}

func TestNewDynamicInputRejectsUnknownExpiration(t *testing.T) {
	var settings config.DynamicInputConfig
	settings.Expiration.Type = "typo"
	_, err := NewDynamicInput("test", settings)
	if err == nil || !strings.Contains(err.Error(), "expiration.type") {
		t.Fatalf("NewDynamicInput() error = %v", err)
	}
}
