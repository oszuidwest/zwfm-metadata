package utils

import (
	"net/http"
	"net/http/httptest"
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
