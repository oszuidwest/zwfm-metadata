package utils

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// httpClient is the shared HTTP client for all requests.
var httpClient = &http.Client{Timeout: 10 * time.Second}

// Get performs an HTTP GET request with standard headers.
func Get(ctx context.Context, rawURL string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, http.NoBody)
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

// DoOK executes an HTTP request and reports non-2xx responses as errors including the response body.
func DoOK(req *http.Request) error {
	resp, err := Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck // Best-effort cleanup

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("status %d: %s", resp.StatusCode, body)
	}
	return nil
}

// ValidateHTTPURL checks that a configured URL parses and uses the http or https scheme.
func ValidateHTTPURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL %q: %w", rawURL, err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("URL %q must use http or https scheme, got %q", rawURL, parsed.Scheme)
	}
	return nil
}
