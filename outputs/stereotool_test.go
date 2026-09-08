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
	tests := []struct {
		name       string
		rdsFieldID int
		wantIDs    []int
	}{
		{
			name:    "Stereo Tool 11 default",
			wantIDs: []int{streamingOutputSongFieldID, defaultRDSRadioTextFieldID},
		},
		{
			name:       "Stereo Tool 10 override",
			rdsFieldID: 15046,
			wantIDs:    []int{streamingOutputSongFieldID, 15046},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotIDs []int
			server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				id, err := stereoToolRequestID(r)
				if err != nil {
					t.Errorf("decode StereoTool request: %v", err)
					return
				}
				gotIDs = append(gotIDs, id)
			}))
			defer server.Close()

			settings := stereoToolTestSettings(t, server.URL)
			settings.RDSFieldID = tt.rdsFieldID
			output := NewStereoToolOutput("test", settings)

			if err := output.sendToStereoTool("Artist & Title"); err != nil {
				t.Fatalf("sendToStereoTool() error = %v", err)
			}
			if len(gotIDs) != len(tt.wantIDs) {
				t.Fatalf("request count = %d, want %d", len(gotIDs), len(tt.wantIDs))
			}
			for index, wantID := range tt.wantIDs {
				if gotIDs[index] != wantID {
					t.Errorf("request %d field ID = %d, want %d", index, gotIDs[index], wantID)
				}
			}
		})
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
		if id == defaultRDSRadioTextFieldID {
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
	payload := strings.TrimPrefix(r.URL.Path, "/json-1/lis")
	var fields map[string]map[string]string
	if err := json.Unmarshal([]byte(payload), &fields); err != nil {
		return 0, fmt.Errorf("decode request path %q: %w", r.URL.Path, err)
	}
	if len(fields) != 1 {
		return 0, fmt.Errorf("field count = %d, want 1", len(fields))
	}
	for rawID := range fields {
		id, err := strconv.Atoi(rawID)
		if err != nil {
			return 0, fmt.Errorf("parse field ID %q: %w", rawID, err)
		}
		return id, nil
	}
	return 0, fmt.Errorf("missing field ID")
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
