package outputs

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"zwfm-metadata/config"
	"zwfm-metadata/core"
)

func TestURLOutput_SendsBearerToken(t *testing.T) {
	var authorization string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authorization = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)

	output, err := NewURLOutput("test", config.URLOutputConfig{
		URL:         server.URL,
		Method:      http.MethodPost,
		BearerToken: "secret",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := output.Send(&core.StructuredText{Title: "Test"}); err != nil {
		t.Fatal(err)
	}

	if authorization != "Bearer secret" {
		t.Errorf("Authorization header = %q, want %q", authorization, "Bearer secret")
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

func TestURLOutputEscapesAfterTemplateFunctions(t *testing.T) {
	tests := []struct {
		template string
		want     string
	}{
		{template: `{{.title | upper}}`, want: "+CAF%C3%89INE+"},
		{template: `{{.title | trim}}`, want: "Caf%C3%A9ine"},
		{template: `{{printf "%.5s" .title}}`, want: "+Caf%C3%A9"},
		{template: `{{len .title}}`, want: "10"},
		{template: `{{$t := .title}}{{$t}}`, want: "+Caf%C3%A9ine+"},
		{template: `{{if .title}}{{.title}}{{end}}`, want: "+Caf%C3%A9ine+"},
		{template: `{{if .artist}}x{{else}}{{.title}}{{end}}`, want: "+Caf%C3%A9ine+"},
		{template: `{{with .title}}{{.}}{{end}}`, want: "+Caf%C3%A9ine+"},
		{template: `{{with .artist}}x{{else}}{{.title}}{{end}}`, want: "+Caf%C3%A9ine+"},
		{template: `{{range 1}}{{$.title}}{{end}}`, want: "+Caf%C3%A9ine+"},
		{template: `{{range 0}}x{{else}}{{.title}}{{end}}`, want: "+Caf%C3%A9ine+"},
		{template: `{{define "t"}}{{.title}}{{end}}{{template "t" .}}`, want: "+Caf%C3%A9ine+"},
	}
	for _, tt := range tests {
		t.Run(tt.template, func(t *testing.T) {
			var rawQuery string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				rawQuery = r.URL.RawQuery
				w.WriteHeader(http.StatusNoContent)
			}))
			t.Cleanup(server.Close)

			output, err := NewURLOutput("test", config.URLOutputConfig{URL: server.URL + "?t=" + tt.template, Method: http.MethodGet})
			if err != nil {
				t.Fatal(err)
			}
			if err := output.Send(&core.StructuredText{Title: " Caféine "}); err != nil {
				t.Fatal(err)
			}
			if want := "t=" + tt.want; rawQuery != want {
				t.Errorf("raw query = %q, want %q", rawQuery, want)
			}
		})
	}
}

func TestURLOutputEncodesLiteralQuerySpaces(t *testing.T) {
	tests := []struct {
		name   string
		method string
		query  string
		want   string
	}{
		{name: "template", method: http.MethodGet, query: "t={{.title}} - x", want: "t=Song%20-%20x"},
		{name: "static", method: http.MethodGet, query: "t=a b", want: "t=a%20b"},
		{name: "post", method: http.MethodPost, query: "t=a b", want: "t=a%20b"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var rawQuery string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				rawQuery = r.URL.RawQuery
				w.WriteHeader(http.StatusNoContent)
			}))
			t.Cleanup(server.Close)

			output, err := NewURLOutput("test", config.URLOutputConfig{URL: server.URL + "?" + tt.query, Method: tt.method})
			if err != nil {
				t.Fatal(err)
			}
			if err := output.Send(&core.StructuredText{Title: "Song"}); err != nil {
				t.Fatal(err)
			}
			if rawQuery != tt.want {
				t.Errorf("raw query = %q, want %q", rawQuery, tt.want)
			}
		})
	}
}
