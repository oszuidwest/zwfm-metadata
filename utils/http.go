package utils

import (
	"context"
	"net/http"
	"time"
)

// httpClient is the shared HTTP client for all requests.
var httpClient = &http.Client{Timeout: 10 * time.Second}

// Get performs an HTTP GET request with standard headers.
func Get(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", UserAgent())
	return httpClient.Do(req) //nolint:gosec // URL is from validated user configuration
}

// Do executes an HTTP request with standard configuration.
func Do(req *http.Request) (*http.Response, error) {
	req.Header.Set("User-Agent", UserAgent())
	return httpClient.Do(req) //nolint:gosec // URL is from validated user configuration
}
