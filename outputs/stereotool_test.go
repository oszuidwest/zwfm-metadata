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

	// Pins the wire contract: Streaming Output Song and FM RDS Radio Text, each as
	// {"<id>":{"forced":"1","new_value":...}} percent-encoded with spaces as %20.
	requestURI := func(id string) string {
		return "/json-1/lis%7B%22" + id + "%22%3A%7B%22forced%22%3A%221%22%2C" +
			"%22new_value%22%3A%22Artist%2FTitle%20%26%20More%20%2B%20100%25%3F%22%7D%7D"
	}
	gotRequestURIsMu.Lock()
	got := slices.Sorted(slices.Values(gotRequestURIs))
	gotRequestURIsMu.Unlock()
	if want := []string{requestURI("6751"), requestURI("9985")}; !slices.Equal(got, want) {
		t.Errorf("request URIs = %q, want %q", got, want)
	}
}

func TestStereoToolOutput_ReturnsFieldErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "unknown field", http.StatusBadRequest)
	}))
	defer server.Close()

	err := NewStereoToolOutput("test", stereoToolTestSettings(server)).sendToStereoTool("Artist - Title")
	for _, field := range stereoToolFields {
		if err == nil || !strings.Contains(err.Error(), field.name) {
			t.Errorf("sendToStereoTool() error = %v, want an error for %s", err, field.name)
		}
	}
}

func stereoToolTestSettings(server *httptest.Server) config.StereoToolOutputConfig {
	addr := server.Listener.Addr().(*net.TCPAddr) // httptest always listens on TCP
	return config.StereoToolOutputConfig{Hostname: addr.IP.String(), Port: addr.Port}
}
