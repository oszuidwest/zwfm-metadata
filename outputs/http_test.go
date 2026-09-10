package outputs

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"zwfm-metadata/config"
	"zwfm-metadata/core"
)

func TestHTTPOutputEndpoints(t *testing.T) {
	output, err := NewHTTPOutput("test", config.HTTPOutputConfig{Endpoints: []config.HTTPEndpoint{
		{Path: "/metadata.json", ResponseType: "json"},
		{Path: "/metadata.xml", ResponseType: "xml"},
		{Path: "/metadata.txt", ResponseType: "plaintext"},
		{Path: "/mapped", ResponseType: "json", PayloadMapping: map[string]any{"track": "{{.title}}", "station": "Test"}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	output.RegisterRoutes(mux)

	beforeSend := httptest.NewRecorder()
	mux.ServeHTTP(beforeSend, httptest.NewRequest(http.MethodGet, "/metadata.json", http.NoBody))
	if beforeSend.Code != http.StatusNoContent {
		t.Fatalf("status before Send = %d, want %d", beforeSend.Code, http.StatusNoContent)
	}

	metadata := &core.Metadata{Artist: "Artist", Title: "Title", UpdatedAt: time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)}
	if err := output.Send(core.NewStructuredText(metadata)); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		path        string
		contentType string
		body        string
	}{
		{path: "/metadata.json", contentType: "application/json", body: `"title":"Title"`},
		{path: "/metadata.xml", contentType: "application/xml", body: "<title>Title</title>"},
		{path: "/metadata.txt", contentType: "text/plain", body: "Artist - Title"},
		{path: "/mapped", contentType: "application/json", body: `"track":"Title"`},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			response := httptest.NewRecorder()
			mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, tt.path, http.NoBody))
			if response.Code != http.StatusOK || response.Header().Get("Content-Type") != tt.contentType || !strings.Contains(response.Body.String(), tt.body) {
				t.Fatalf("response = status %d, content type %q, body %q", response.Code, response.Header().Get("Content-Type"), response.Body.String())
			}
			if response.Header().Get("Access-Control-Allow-Origin") != "*" {
				t.Error("missing CORS header")
			}
		})
	}
}
