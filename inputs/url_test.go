package inputs

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"zwfm-metadata/config"
	"zwfm-metadata/core"
)

func TestURLInputParseJSON(t *testing.T) {
	wantExpiry := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name       string
		body       string
		jsonKey    string
		expiryKey  string
		wantTitle  string
		wantExpiry *time.Time
		wantOK     bool
	}{
		{name: "nested title and expiry", body: `{"now":{"title":"Song","expiry":"2026-09-11T12:00:00Z"}}`, jsonKey: "now.title", expiryKey: "now.expiry", wantTitle: "Song", wantExpiry: &wantExpiry, wantOK: true},
		{name: "invalid expiry", body: `{"title":"Song","expiry":"later"}`, jsonKey: "title", expiryKey: "expiry", wantTitle: "Song", wantOK: true},
		{name: "missing title", body: `{}`, jsonKey: "title"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := &URLInput{
				InputBase: core.NewInputBase("test"),
				settings:  config.URLInputConfig{JSONKey: tt.jsonKey, ExpiryKey: tt.expiryKey},
			}
			title, expiresAt, ok := input.parseJSON([]byte(tt.body))
			if title != tt.wantTitle || ok != tt.wantOK {
				t.Fatalf("parseJSON() = (%q, %v, %v), want title %q and ok %v", title, expiresAt, ok, tt.wantTitle, tt.wantOK)
			}
			if tt.wantExpiry == nil {
				if expiresAt != nil {
					t.Fatalf("expiresAt = %v, want nil", expiresAt)
				}
			} else if expiresAt == nil || !expiresAt.Equal(*tt.wantExpiry) {
				t.Fatalf("expiresAt = %v, want %v", expiresAt, tt.wantExpiry)
			}
		})
	}
}

func TestURLInputPollRejectsUnsuccessfulStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "bad gateway", http.StatusBadGateway)
	}))
	t.Cleanup(server.Close)

	input, err := NewURLInput("test", &config.URLInputConfig{URL: server.URL, PollingInterval: 1})
	if err != nil {
		t.Fatal(err)
	}
	input.poll(context.Background())
	if input.GetMetadata() != nil {
		t.Fatal("unsuccessful response replaced metadata")
	}
}

func TestURLInputStartCancelsActiveRequest(t *testing.T) {
	requestStarted := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		close(requestStarted)
		<-r.Context().Done()
	}))
	t.Cleanup(server.Close)

	input, err := NewURLInput("test", &config.URLInputConfig{URL: server.URL, PollingInterval: 1})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- input.Start(ctx)
	}()

	select {
	case <-requestStarted:
	case <-time.After(time.Second):
		t.Fatal("request did not start")
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Start() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Start() did not stop after cancellation")
	}
}
