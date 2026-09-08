package outputs

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"zwfm-metadata/config"
)

func TestStereoToolOutput_FieldIDs(t *testing.T) {
	var gotIDs []int
	var gotMetadata []string
	var gotRequestURIs []string
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		id, fields, err := stereoToolRequestField(r)
		if err != nil {
			t.Errorf("decode StereoTool request: %v", err)
			return
		}
		gotIDs = append(gotIDs, id)
		gotMetadata = append(gotMetadata, fields["new_value"])
		gotRequestURIs = append(gotRequestURIs, r.RequestURI)
	}))
	defer server.Close()

	output := NewStereoToolOutput("test", stereoToolTestSettings(t, server.URL))
	metadata := "Artist/Title & More + 100%?"
	if err := output.sendToStereoTool(metadata); err != nil {
		t.Fatalf("sendToStereoTool() error = %v", err)
	}

	wantIDs := []int{stereoTool11SongFieldID, stereoTool11RadioTextFieldID}
	if len(gotIDs) != len(wantIDs) {
		t.Fatalf("request count = %d, want %d", len(gotIDs), len(wantIDs))
	}
	for index, wantID := range wantIDs {
		if gotIDs[index] != wantID {
			t.Errorf("request %d field ID = %d, want %d", index, gotIDs[index], wantID)
		}
		if gotMetadata[index] != metadata {
			t.Errorf("request %d metadata = %q, want %q", index, gotMetadata[index], metadata)
		}
		if strings.Contains(gotRequestURIs[index], "Artist/Title") {
			t.Errorf("request %d URI contains an unescaped slash: %q", index, gotRequestURIs[index])
		}
	}
}

func TestStereoToolOutput_ReturnsFieldError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, err := stereoToolRequestID(r)
		if err != nil {
			t.Errorf("decode StereoTool request: %v", err)
			http.Error(w, "invalid request", http.StatusInternalServerError)
			return
		}
		if id == stereoTool11RadioTextFieldID {
			http.Error(w, "unknown field", http.StatusBadRequest)
			return
		}
	}))
	defer server.Close()

	output := NewStereoToolOutput("test", stereoToolTestSettings(t, server.URL))
	err := output.sendToStereoTool("Artist - Title")
	if err == nil {
		t.Fatal("sendToStereoTool() error = nil, want an error")
	}
	if !strings.Contains(err.Error(), "FM RDS Radio Text") || !strings.Contains(err.Error(), "status 400") {
		t.Fatalf("sendToStereoTool() error = %q", err)
	}
}

func stereoToolRequestID(r *http.Request) (int, error) {
	id, _, err := stereoToolRequestField(r)
	return id, err
}

func stereoToolRequestField(r *http.Request) (int, map[string]string, error) {
	payload := strings.TrimPrefix(r.URL.Path, "/json-1/lis")
	var fields map[string]map[string]string
	if err := json.Unmarshal([]byte(payload), &fields); err != nil {
		return 0, nil, fmt.Errorf("decode request path %q: %w", r.URL.Path, err)
	}
	if len(fields) != 1 {
		return 0, nil, fmt.Errorf("field count = %d, want 1", len(fields))
	}
	for rawID, values := range fields {
		id, err := strconv.Atoi(rawID)
		if err != nil {
			return 0, nil, fmt.Errorf("parse field ID %q: %w", rawID, err)
		}
		return id, values, nil
	}
	return 0, nil, fmt.Errorf("missing field ID")
}

func stereoToolTestSettings(t *testing.T, serverURL string) config.StereoToolOutputConfig {
	t.Helper()

	parsedURL, err := url.Parse(serverURL)
	if err != nil {
		t.Fatalf("parse test server URL: %v", err)
	}
	hostname, rawPort, err := net.SplitHostPort(parsedURL.Host)
	if err != nil {
		t.Fatalf("split test server address: %v", err)
	}
	port, err := strconv.Atoi(rawPort)
	if err != nil {
		t.Fatalf("parse test server port: %v", err)
	}

	return config.StereoToolOutputConfig{Hostname: hostname, Port: port}
}
