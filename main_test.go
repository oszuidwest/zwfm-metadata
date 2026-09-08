package main

import (
	"testing"

	"zwfm-metadata/config"
	"zwfm-metadata/core"
)

func TestSetupOutputTiming(t *testing.T) {
	tests := []struct {
		name           string
		settings       map[string]any
		expectedTiming config.OutputTiming
		wantError      bool
	}{
		{
			name: "configured timings",
			settings: map[string]any{
				"delay":         12,
				"fallbackDelay": 5,
				"filename":      "unused",
			},
			expectedTiming: config.OutputTiming{Delay: 12, FallbackDelay: 5},
		},
		{
			name: "default fallback delay",
			settings: map[string]any{
				"delay":    12,
				"filename": "unused",
			},
			expectedTiming: config.OutputTiming{Delay: 12},
		},
		{
			name: "invalid delay",
			settings: map[string]any{
				"delay":    "invalid",
				"filename": "unused",
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := core.NewMetadataRouter()
			outputCfg := config.OutputConfig{
				Type:     "file",
				Name:     "test-output",
				Settings: tt.settings,
			}

			err := setupOutput(router, &outputCfg)
			if tt.wantError {
				if err == nil {
					t.Fatal("setupOutput() error = nil, want an error")
				}
				return
			}
			if err != nil {
				t.Fatalf("setupOutput() error = %v", err)
			}

			if got := router.GetOutputTiming(outputCfg.Name); got != tt.expectedTiming {
				t.Errorf("GetOutputTiming() = %+v, want %+v", got, tt.expectedTiming)
			}
		})
	}
}
