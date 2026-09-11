package outputs

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"zwfm-metadata/config"
	"zwfm-metadata/core"
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

func TestURLOutputEscapesTemplateValuesByURLComponent(t *testing.T) {
	var escapedPath, rawQuery, title string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		escapedPath = r.URL.EscapedPath()
		rawQuery = r.URL.RawQuery
		title = r.URL.Query().Get("title")
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)

	output, err := NewURLOutput("test", config.URLOutputConfig{
		URL:    server.URL + "/metadata/{{.title}}?title={{.title}}",
		Method: http.MethodGet,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := output.Send(&core.StructuredText{Title: "Foo Bar/Baz+Q"}); err != nil {
		t.Fatal(err)
	}

	if escapedPath != "/metadata/Foo%20Bar%2FBaz+Q" {
		t.Errorf("escaped path = %q, want %q", escapedPath, "/metadata/Foo%20Bar%2FBaz+Q")
	}
	if rawQuery != "title=Foo+Bar%2FBaz%2BQ" {
		t.Errorf("raw query = %q, want %q", rawQuery, "title=Foo+Bar%2FBaz%2BQ")
	}
	if title != "Foo Bar/Baz+Q" {
		t.Errorf("title query value = %q, want %q", title, "Foo Bar/Baz+Q")
	}
}
