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

const maxErrorBodyBytes = 4 << 10

var httpClient = &http.Client{
	Timeout: 10 * time.Second,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return errors.New("stopped after 10 redirects")
		}
		if len(via) > 0 && strings.HasPrefix(via[0].Header.Get("Authorization"), "Bearer ") &&
			req.URL.Scheme != "https" {
			return errors.New("refusing to redirect bearer token to non-https url")
		}
		return nil
	},
}

// Get issues an HTTP GET through the shared client.
func Get(ctx context.Context, rawURL string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, http.NoBody)
	if err != nil {
		return nil, err
	}
	return do(req)
}

func do(req *http.Request) (*http.Response, error) {
	req.Header.Set("User-Agent", UserAgent())
	return httpClient.Do(req) //nolint:gosec // Targets are supplied by the operator.
}

// DoOK reports non-2xx responses with up to 4 KiB of response body.
func DoOK(req *http.Request) error {
	resp, err := do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck // Best-effort cleanup

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodyBytes))
		if readErr != nil {
			return fmt.Errorf("status %d: read response body: %w", resp.StatusCode, readErr)
		}
		return fmt.Errorf("status %d: %s", resp.StatusCode, body)
	}
	return nil
}

// ValidateHTTPURL accepts absolute HTTP and HTTPS URLs.
func ValidateHTTPURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid url %q: %w", rawURL, err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("url %q must use http or https scheme, got %q", rawURL, parsed.Scheme)
	}
	if parsed.Host == "" {
		return fmt.Errorf("url %q must include a host", rawURL)
	}
	return nil
}
