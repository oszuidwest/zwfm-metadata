package utils

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// httpClient is the shared HTTP client for all requests.
var httpClient = &http.Client{
	Timeout: 10 * time.Second,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return errors.New("stopped after 10 redirects")
		}
		if len(via) > 0 && strings.HasPrefix(via[0].Header.Get("Authorization"), "Bearer ") &&
			req.URL.Scheme != "https" {
			return errors.New("refusing to redirect bearer token to non-HTTPS URL")
		}
		return nil
	},
}

// Get performs an HTTP GET request with standard headers.
func Get(ctx context.Context, rawURL string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, http.NoBody)
	if err != nil {
		return nil, err
	}
	return do(req)
}

// do executes an HTTP request with the shared client and User-Agent.
func do(req *http.Request) (*http.Response, error) {
	req.Header.Set("User-Agent", UserAgent())
	return httpClient.Do(req) //nolint:gosec // URL is from validated user configuration
}

// DoOK executes an HTTP request and reports non-2xx responses as errors including the response body.
func DoOK(req *http.Request) error {
	resp, err := do(req)
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
