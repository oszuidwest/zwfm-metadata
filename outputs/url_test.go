package outputs

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"zwfm-metadata/config"
	"zwfm-metadata/core"
)

func TestURLOutput_BearerTokenRequiresHTTPS(t *testing.T) {
	requests := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		requests <- struct{}{}
	}))
	defer server.Close()

	output, err := NewURLOutput("test", config.URLOutputConfig{
		URL:         server.URL,
		Method:      http.MethodPost,
		BearerToken: "secret",
	})
	if err != nil {
		t.Fatalf("NewURLOutput() error = %v", err)
	}

	output.Send(&core.StructuredText{Title: "Title"})

	select {
	case <-requests:
		t.Fatal("bearer-token request was sent over HTTP")
	default:
	}
}
