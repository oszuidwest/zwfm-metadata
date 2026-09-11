package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

// ParseJSONSettings decodes a component's raw settings into T. Absent settings yield the zero value.
func ParseJSONSettings[T any](settings json.RawMessage) (T, error) {
	var result T
	if len(settings) == 0 {
		return result, nil
	}
	decoder := json.NewDecoder(bytes.NewReader(settings))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&result); err != nil {
		return result, fmt.Errorf("failed to parse settings: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return result, fmt.Errorf("failed to parse settings: expected a single JSON value")
	}
	return result, nil
}
