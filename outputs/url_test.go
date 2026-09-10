package outputs

import (
	"testing"

	"zwfm-metadata/config"
)

func TestNewURLOutput_BearerTokenRequiresHTTPS(t *testing.T) {
	_, err := NewURLOutput("test", config.URLOutputConfig{
		URL:         "http://example.com/metadata",
		Method:      "POST",
		BearerToken: "secret",
	})
	if err == nil {
		t.Fatal("NewURLOutput() error = nil, want HTTPS requirement error")
	}
}
