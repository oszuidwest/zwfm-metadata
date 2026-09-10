package main

import (
	"encoding/json"
	"testing"

	"zwfm-metadata/config"
	"zwfm-metadata/core"
	"zwfm-metadata/inputs"
)

func TestSetupOutputTiming(t *testing.T) {
	tests := []struct {
		name           string
		settings       json.RawMessage
		expectedTiming core.OutputTiming
		wantError      bool
	}{
		{
			name:           "configured timings",
			settings:       json.RawMessage(`{"delay": 12, "fallbackDelay": 5}`),
			expectedTiming: core.OutputTiming{Delay: 12, FallbackDelay: 5},
		},
		{
			name:           "default fallback delay",
			settings:       json.RawMessage(`{"delay": 12}`),
			expectedTiming: core.OutputTiming{Delay: 12},
		},
		{
			name:      "invalid delay",
			settings:  json.RawMessage(`{"delay": "invalid"}`),
			wantError: true,
		},
		{
			name:      "negative delay",
			settings:  json.RawMessage(`{"delay": -1}`),
			wantError: true,
		},
		{
			name:      "negative fallback delay",
			settings:  json.RawMessage(`{"delay": 12, "fallbackDelay": -1}`),
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := core.NewMetadataRouter()
			input := inputs.NewTextInput("input", config.TextInputConfig{Text: "test"})
			if err := router.AddInput(input, &core.InputSpec{}); err != nil {
				t.Fatalf("AddInput() error = %v", err)
			}
			outputCfg := config.OutputConfig{
				Type:     "file",
				Name:     "test-output",
				Inputs:   []string{"input"},
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

			if got := router.GetOutputStatus()[0].OutputTiming; got != tt.expectedTiming {
				t.Errorf("output timing = %+v, want %+v", got, tt.expectedTiming)
			}
		})
	}
}

func TestExampleConfigComponents(t *testing.T) {
	appConfig, err := config.LoadConfig("config-example.json")
	if err != nil {
		t.Fatal(err)
	}
	router := core.NewMetadataRouter()
	for i := range appConfig.Inputs {
		if err := setupInput(router, &appConfig.Inputs[i]); err != nil {
			t.Fatalf("setup input %q: %v", appConfig.Inputs[i].Name, err)
		}
	}
	for i := range appConfig.Outputs {
		if err := setupOutput(router, &appConfig.Outputs[i]); err != nil {
			t.Fatalf("setup output %q: %v", appConfig.Outputs[i].Name, err)
		}
	}
}
