package main

import (
	"testing"

	"zwfm-metadata/config"
	"zwfm-metadata/core"
)

func TestSetupOutputRejectsNegativeFallbackDelay(t *testing.T) {
	err := setupOutput(core.NewMetadataRouter(), &config.OutputConfig{
		Type:     "file",
		Name:     "test-output",
		Settings: map[string]any{"fallbackDelay": -1},
	})
	if err == nil {
		t.Fatal("setupOutput() error = nil, want an error")
	}
}
