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

const stereoToolStored = `{ "9985": { "enabled": "1", "active": "1", "value_text": "x", "value": "x", "type": "string", "text": "RadioText"} }`

func TestStereoToolOutput_FieldIDs(t *testing.T) {
	var gotRequestURIsMu sync.Mutex
	var gotRequestURIs []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRequestURIsMu.Lock()
		defer gotRequestURIsMu.Unlock()
		gotRequestURIs = append(gotRequestURIs, r.RequestURI)
		_, _ = w.Write([]byte(stereoToolStored))
	}))
	defer server.Close()

	output := NewStereoToolOutput("test", stereoToolTestSettings(server))
	if err := output.sendToStereoTool("Artist/Title & More + 100%?"); err != nil {
		t.Fatalf("sendToStereoTool() error = %v", err)
	}

	// Pins the wire contract: Streaming Output Song and FM RDS Radio Text, each as
	// {"<id>":{"forced":"1","new_value":...}} percent-encoded with spaces as %20.
	// Only RadioText escapes the "/", as %5C%2F.
	requestURI := func(id, slash string) string {
		return "/json-1/lis%7B%22" + id + "%22%3A%7B%22forced%22%3A%221%22%2C" +
			"%22new_value%22%3A%22Artist" + slash + "Title%20%26%20More%20%2B%20100%25%3F%22%7D%7D"
	}
	gotRequestURIsMu.Lock()
	got := slices.Sorted(slices.Values(gotRequestURIs))
	gotRequestURIsMu.Unlock()
	if want := []string{requestURI("6751", "%2F"), requestURI("9985", "%5C%2F")}; !slices.Equal(got, want) {
		t.Errorf("request URIs = %q, want %q", got, want)
	}
}

func TestStereoToolOutput_SendsValuesLiterally(t *testing.T) {
	tests := []struct {
		metadata string
		wantSong string
		wantRT   string
	}{
		{metadata: `A/B: <C> D|E\F`, wantSong: `"A/B: <C> D|E\F"`, wantRT: `"A\/B\: \<C\> D\|E\\F"`},
		{metadata: `Say "Hello"`, wantSong: `'Say "Hello"'`, wantRT: `'Say "Hello"'`},
		{metadata: `Don't "Stop"`, wantSong: `"Don't 'Stop'"`, wantRT: `"Don't 'Stop'"`},
	}
	for _, tt := range tests {
		t.Run(tt.metadata, func(t *testing.T) {
			var mu sync.Mutex
			var got []string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				defer mu.Unlock()
				got = append(got, r.URL.Path)
				_, _ = w.Write([]byte(stereoToolStored))
			}))
			defer server.Close()

			if err := NewStereoToolOutput("test", stereoToolTestSettings(server)).sendToStereoTool(tt.metadata); err != nil {
				t.Fatalf("sendToStereoTool() error = %v", err)
			}

			mu.Lock()
			defer mu.Unlock()
			for _, want := range []string{
				`/json-1/lis{"6751":{"forced":"1","new_value":` + tt.wantSong + `}}`,
				`/json-1/lis{"9985":{"forced":"1","new_value":` + tt.wantRT + `}}`,
			} {
				if !slices.Contains(got, want) {
					t.Errorf("no request for %q, got %q", want, got)
				}
			}
		})
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

func TestStereoToolOutput_ChecksResponseBody(t *testing.T) {
	const (
		unknownID   = `{ "6751": "ERROR_UNKNOWN_ID" }`
		parserError = `{ "error 0": "ERROR_REACHED_END_WITHOUT_CLOSING_QUOTE", "error 1": "ERROR_NOT_ALL_BRACKETS_MATCHED" }`
	)
	tests := []struct {
		name      string
		song      string
		radioText string
		wantErr   string
	}{
		{name: "both stored", song: stereoToolStored, radioText: stereoToolStored},
		{name: "plugin build without Song", song: unknownID, radioText: stereoToolStored},
		{name: "unknown RadioText", song: stereoToolStored, radioText: unknownID, wantErr: "FM RDS Radio Text"},
		{name: "refused RadioText", song: stereoToolStored, radioText: parserError, wantErr: "ERROR_REACHED_END"},
		{name: "refused Song", song: parserError, radioText: stereoToolStored, wantErr: "Streaming Output Song"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body := tt.radioText
				if strings.Contains(r.URL.Path, "6751") {
					body = tt.song
				}
				_, _ = w.Write([]byte(body))
			}))
			defer server.Close()

			err := NewStereoToolOutput("test", stereoToolTestSettings(server)).sendToStereoTool("Artist - Title")
			switch {
			case tt.wantErr == "" && err != nil:
				t.Fatalf("sendToStereoTool() error = %v", err)
			case tt.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErr)):
				t.Fatalf("sendToStereoTool() error = %v, want it to contain %q", err, tt.wantErr)
			}
		})
	}
}

func stereoToolTestSettings(server *httptest.Server) config.StereoToolOutputConfig {
	addr := server.Listener.Addr().(*net.TCPAddr) // httptest always listens on TCP
	return config.StereoToolOutputConfig{Hostname: addr.IP.String(), Port: addr.Port}
}
