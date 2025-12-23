package utils

import (
	"os"
	"path/filepath"
)

// WriteFile writes content to a file, creating directories if needed.
func WriteFile(filename string, content []byte) error {
	cleanPath := filepath.Clean(filename)

	if dir := filepath.Dir(cleanPath); dir != "." {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return err
		}
	}

	return os.WriteFile(cleanPath, content, 0o600)
}
