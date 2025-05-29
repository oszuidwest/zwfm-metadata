package utils

import (
	"io"
	"net/http"
	"os"
	"time"
)

// CreateHTTPClient creates a standard HTTP client with common settings
func CreateHTTPClient(timeout time.Duration) *http.Client {
	if timeout == 0 {
		timeout = 10 * time.Second
	}
	return &http.Client{
		Timeout: timeout,
	}
}

// CloseBody safely closes an HTTP response body with error logging
func CloseBody(body io.ReadCloser) {
	if err := body.Close(); err != nil {
		LogError("Failed to close HTTP response body: %v", err)
	}
}

// CloseFile safely closes a file with error logging
func CloseFile(file *os.File) {
	if err := file.Close(); err != nil {
		LogError("Failed to close file: %v", err)
	}
}
