package utils

import (
	"os"
	"path/filepath"
)

// WriteFile writes content to a file, creating parent directories if needed.
func WriteFile(filename string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(filename), 0o750); err != nil {
		return err
	}
	return os.WriteFile(filename, content, 0o600)
}
