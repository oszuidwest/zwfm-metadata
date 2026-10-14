package outputs

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"zwfm-metadata/config"
)

func TestNewWebSocketOutputRejectsNegativeHeartbeatInterval(t *testing.T) {
	_, err := NewWebSocketOutput("ws", config.WebSocketOutputConfig{Path: "/ws", HeartbeatInterval: -1})
	if err == nil || !strings.Contains(err.Error(), "heartbeatInterval") {
		t.Fatalf("NewWebSocketOutput() error = %v, want a heartbeatInterval error", err)
	}
}

func TestWebSocketOutputSendsUnmappedHeartbeat(t *testing.T) {
	output, err := NewWebSocketOutput("ws", config.WebSocketOutputConfig{
		Path:              "/ws",
		HeartbeatInterval: 1,
		PayloadMapping:    map[string]any{"event": "{{.type}}"},
	})
	if err != nil {
		t.Fatal(err)
	}

	mux := http.NewServeMux()
	output.RegisterRoutes(mux)
	server := httptest.NewServer(mux)
	defer server.Close()

	conn, resp, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"/ws", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close() //nolint:errcheck // Best-effort cleanup
	if resp != nil && resp.Body != nil {
		resp.Body.Close() //nolint:errcheck,gosec // Best-effort cleanup
	}

	if err := conn.SetReadDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatal(err)
	}
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	if got := string(msg); got != `{"type":"heartbeat"}` {
		t.Fatalf("message = %s, want {\"type\":\"heartbeat\"}", got)
	}
}
