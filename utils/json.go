package utils

import (
	"encoding/json"
	"fmt"
)

// ParseJSONSettings is a generic function to parse JSON settings.
func ParseJSONSettings[T any](settings interface{}) (*T, error) {
	var result T

	// Handle different input types
	var settingsJSON []byte
	var err error

	switch v := settings.(type) {
	case json.RawMessage:
		settingsJSON = v
	case map[string]interface{}:
		settingsJSON, err = json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal settings: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported settings type: %T", settings)
	}

	if err := json.Unmarshal(settingsJSON, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal settings: %w", err)
	}

	return &result, nil
}
