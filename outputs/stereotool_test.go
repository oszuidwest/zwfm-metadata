package outputs

import (
	"net"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"

	"zwfm-metadata/config"
)

func TestStereoToolOutput_FieldIDs(t *testing.T) {
	var gotRequestURIsMu sync.Mutex
	var gotRequestURIs []string
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		gotRequestURIsMu.Lock()
		defer gotRequestURIsMu.Unlock()
		gotRequestURIs = append(gotRequestURIs, r.RequestURI)
	}))
	defer server.Close()

	output := NewStereoToolOutput("test", stereoToolTestSettings(server))
	if err := output.sendToStereoTool("Artist/Title & More + 100%?"); err != nil {
		t.Fatalf("sendToStereoTool() error = %v", err)
	}

	// Pins the wire contract: Streaming Output Song, then FM RDS Radio Text, each as
	// {"<id>":{"forced":"1","new_value":...}} with every reserved character escaped
	// and spaces as %20, because Stereo Tool decodes the path query-style.
	requestURI := func(id string) string {
		return "/json-1/lis%7B%22" + id + "%22%3A%7B%22forced%22%3A%221%22%2C" +
			"%22new_value%22%3A%22Artist%2FTitle%20%26%20More%20%2B%20100%25%3F%22%7D%7D"
	}
	gotRequestURIsMu.Lock()
	got := slices.Clone(gotRequestURIs)
	gotRequestURIsMu.Unlock()
	if want := []string{requestURI("6751"), requestURI("9985")}; !slices.Equal(got, want) {
		t.Errorf("request URIs = %q, want %q", got, want)
	}
}

func TestStereoToolOutput_ReturnsFieldError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, `"9985"`) { // FM RDS Radio Text
			http.Error(w, "unknown field", http.StatusBadRequest)
			return
		}
	}))
	defer server.Close()

	output := NewStereoToolOutput("test", stereoToolTestSettings(server))
	err := output.sendToStereoTool("Artist - Title")
	if err == nil {
		t.Fatal("sendToStereoTool() error = nil, want an error")
	}
	if !strings.Contains(err.Error(), "FM RDS Radio Text") || !strings.Contains(err.Error(), "status 400") {
		t.Fatalf("sendToStereoTool() error = %q", err)
	}
}

func stereoToolTestSettings(server *httptest.Server) config.StereoToolOutputConfig {
	addr := server.Listener.Addr().(*net.TCPAddr) // httptest always listens on TCP
	return config.StereoToolOutputConfig{Hostname: addr.IP.String(), Port: addr.Port}
}
