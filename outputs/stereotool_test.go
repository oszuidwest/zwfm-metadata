package outputs

import (
	"encoding/json"
	"fmt"
	"maps"
	"net"
	"net/http"
	"net/http/httptest"
	"slices"
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

	output := NewStereoToolOutput("test", stereoToolTestSettings(t, server))
	metadata := "Artist/Title & More + 100%?"
	if err := output.sendToStereoTool(metadata); err != nil {
		t.Fatalf("sendToStereoTool() error = %v", err)
	}

	// Literal IDs pin the wire contract: Streaming Output Song, then FM RDS Radio Text.
	if wantIDs := []int{6751, 9985}; !slices.Equal(gotIDs, wantIDs) {
		t.Errorf("field IDs = %v, want %v", gotIDs, wantIDs)
	}
	if !slices.Equal(gotMetadata, []string{metadata, metadata}) {
		t.Errorf("metadata = %q, want both %q", gotMetadata, metadata)
	}
	for _, uri := range gotRequestURIs {
		if strings.Contains(uri, "Artist/Title") {
			t.Errorf("URI contains an unescaped slash: %q", uri)
		}
	}
}

func TestStereoToolOutput_ReturnsFieldError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, _, err := stereoToolRequestField(r)
		if err != nil {
			t.Errorf("decode StereoTool request: %v", err)
			http.Error(w, "invalid request", http.StatusInternalServerError)
			return
		}
		if id == 9985 { // FM RDS Radio Text
			http.Error(w, "unknown field", http.StatusBadRequest)
			return
		}
	}))
	defer server.Close()

	output := NewStereoToolOutput("test", stereoToolTestSettings(t, server))
	err := output.sendToStereoTool("Artist - Title")
	if err == nil {
		t.Fatal("sendToStereoTool() error = nil, want an error")
	}
	if !strings.Contains(err.Error(), "FM RDS Radio Text") || !strings.Contains(err.Error(), "status 400") {
		t.Fatalf("sendToStereoTool() error = %q", err)
	}
}

func stereoToolRequestField(r *http.Request) (id int, values map[string]string, err error) {
	var fields map[string]map[string]string
	if err := json.Unmarshal([]byte(strings.TrimPrefix(r.URL.Path, "/json-1/lis")), &fields); err != nil {
		return 0, nil, fmt.Errorf("decode request path %q: %w", r.URL.Path, err)
	}
	if len(fields) != 1 {
		return 0, nil, fmt.Errorf("field count = %d, want 1", len(fields))
	}
	rawID := slices.Collect(maps.Keys(fields))[0]
	id, err = strconv.Atoi(rawID)
	if err != nil {
		return 0, nil, fmt.Errorf("parse field ID %q: %w", rawID, err)
	}
	return id, fields[rawID], nil
}

func stereoToolTestSettings(t *testing.T, server *httptest.Server) config.StereoToolOutputConfig {
	t.Helper()

	addr, ok := server.Listener.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatalf("test server address is %T, want *net.TCPAddr", server.Listener.Addr())
	}
	return config.StereoToolOutputConfig{Hostname: addr.IP.String(), Port: addr.Port}
}
