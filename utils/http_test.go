package utils

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

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
