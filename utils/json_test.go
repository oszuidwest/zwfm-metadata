package utils

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestParseJSONSettingsRejectsUnknownFields(t *testing.T) {
	type nested struct {
		Type string `json:"type"`
	}
	type settings struct {
		Delay  int    `json:"delay"`
		Nested nested `json:"nested"`
	}

	tests := []struct {
		name     string
		settings json.RawMessage
		field    string
	}{
		{name: "top level", settings: json.RawMessage(`{"dealy":12}`), field: "dealy"},
		{name: "nested", settings: json.RawMessage(`{"nested":{"tpye":"x"}}`), field: "tpye"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseJSONSettings[settings](tt.settings)
			if err == nil || !strings.Contains(err.Error(), `unknown field "`+tt.field+`"`) {
				t.Fatalf("ParseJSONSettings() error = %v, want unknown field %q", err, tt.field)
			}
		})
	}
}
