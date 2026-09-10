package utils

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHTTPClientRejectsBearerDowngradeRedirect(t *testing.T) {
	original := httptest.NewRequest(http.MethodGet, "https://example.com", http.NoBody)
	original.Header.Set("Authorization", "Bearer secret")
	redirect := httptest.NewRequest(http.MethodGet, "http://example.com", http.NoBody)

	if err := httpClient.CheckRedirect(redirect, []*http.Request{original}); err == nil {
		t.Fatal("CheckRedirect() error = nil, want downgrade redirect error")
	}
}

func TestDoOKLimitsErrorResponseBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		if _, err := w.Write([]byte(strings.Repeat("x", maxErrorBodyBytes*2))); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, server.URL, http.NoBody)
	if err != nil {
		t.Fatal(err)
	}
	err = DoOK(req)
	if err == nil {
		t.Fatal("DoOK() error = nil, want status error")
	}
	if !strings.Contains(err.Error(), "status 502") {
		t.Fatalf("DoOK() error = %q, want status 502", err)
	}
	if len(err.Error()) > maxErrorBodyBytes+100 {
		t.Fatalf("DoOK() error length = %d, want bounded response body", len(err.Error()))
	}
}

func TestValidateHTTPURLRequiresHost(t *testing.T) {
	if err := ValidateHTTPURL("https:"); err == nil {
		t.Fatal("ValidateHTTPURL() error = nil, want missing host error")
	}
}
