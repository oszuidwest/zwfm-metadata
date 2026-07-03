package utils

import (
	"encoding/json"
	"fmt"
)

// ParseJSONSettings parses JSON-decoded settings into the specified type T.
func ParseJSONSettings[T any](settings map[string]any) (*T, error) {
	settingsJSON, err := json.Marshal(settings)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal settings: %w", err)
	}

	var result T
	if err := json.Unmarshal(settingsJSON, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal settings: %w", err)
	}

	return &result, nil
}
