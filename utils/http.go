package utils

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const maxBodyBytes = 4 << 10

var httpClient = &http.Client{Timeout: 10 * time.Second}

// noRedirectClient returns redirects as is, so the target cannot send the request
// on to another address.
var noRedirectClient = &http.Client{
	Timeout:       httpClient.Timeout,
	CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
}

// Get issues an HTTP GET that does not follow redirects.
func Get(ctx context.Context, rawURL string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, http.NoBody)
	if err != nil {
		return nil, err
	}
	return do(noRedirectClient, req)
}

func do(client *http.Client, req *http.Request) (*http.Response, error) {
	req.Header.Set("User-Agent", UserAgent())
	return client.Do(req) //nolint:gosec // Targets are supplied by the operator.
}

// DoOK reports non-2xx responses with up to 4 KiB of response body.
func DoOK(req *http.Request) error {
	resp, err := doStatusOK(httpClient, req)
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck // Best-effort cleanup
	return nil
}

// DoOKBody is DoOK that also returns up to 4 KiB of a 2xx response body. It does not
// follow redirects, so credentials on the request only go to the requested address.
func DoOKBody(req *http.Request) ([]byte, error) {
	resp, err := doStatusOK(noRedirectClient, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck // Best-effort cleanup

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}
	return body, nil
}

func doStatusOK(client *http.Client, req *http.Request) (*http.Response, error) {
	resp, err := do(client, req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return resp, nil
	}
	defer resp.Body.Close() //nolint:errcheck // Best-effort cleanup

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return nil, fmt.Errorf("status %d: read response body: %w", resp.StatusCode, err)
	}
	return nil, fmt.Errorf("status %d: %s", resp.StatusCode, body)
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
