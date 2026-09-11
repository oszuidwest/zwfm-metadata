package utils

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestParseJSONSettingsRejectsUnknownFields(t *testing.T) {
	type settings struct {
		Delay int `json:"delay"`
	}

	_, err := ParseJSONSettings[settings](json.RawMessage(`{"dealy":12}`))
	if err == nil || !strings.Contains(err.Error(), `unknown field "dealy"`) {
		t.Fatalf("ParseJSONSettings() error = %v, want unknown field error", err)
	}
}

func TestParseJSONSettingsRejectsMultipleValues(t *testing.T) {
	type settings struct {
		Delay int `json:"delay"`
	}

	_, err := ParseJSONSettings[settings](json.RawMessage(`{"delay":12} {"delay":13}`))
	if err == nil || !strings.Contains(err.Error(), "single JSON value") {
		t.Fatalf("ParseJSONSettings() error = %v, want single value error", err)
	}
}
