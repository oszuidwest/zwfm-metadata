package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"
)

const healthcheckTimeout = 3 * time.Second

func runHealthcheck(ctx context.Context, targetURL string) error {
	if targetURL == "" {
		return errors.New("healthcheck URL is required")
	}

	reqCtx, cancel := context.WithTimeout(ctx, healthcheckTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, targetURL, http.NoBody)
	if err != nil {
		return fmt.Errorf("create healthcheck request: %w", err)
	}
	req.Header.Set("User-Agent", "zwfm-metadata-healthcheck")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("execute healthcheck request: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // Healthcheck response close errors do not affect readiness.

	if resp.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("healthcheck returned status %s", resp.Status)
	}

	return nil
}
