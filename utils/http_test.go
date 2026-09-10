package utils

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDoOK_RejectsBearerTokenDowngradeRedirect(t *testing.T) {
	requests := make(chan string, 1)
	target := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, req *http.Request) {
		requests <- req.Header.Get("Authorization")
	}))
	defer target.Close()

	source := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		http.Redirect(w, req, target.URL, http.StatusFound)
	}))
	defer source.Close()

	originalTransport := httpClient.Transport
	httpClient.Transport = source.Client().Transport
	t.Cleanup(func() {
		httpClient.Transport = originalTransport
	})

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, source.URL, http.NoBody)
	if err != nil {
		t.Fatalf("NewRequestWithContext() error = %v", err)
	}
	req.Header.Set("Authorization", "Bearer secret")

	if err := DoOK(req); err == nil {
		t.Fatal("DoOK() error = nil, want downgrade redirect error")
	}
	select {
	case authorization := <-requests:
		t.Fatalf("downgrade target received Authorization %q", authorization)
	default:
	}
}
