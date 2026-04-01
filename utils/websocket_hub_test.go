package utils

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestWebSocketHubBroadcastSerializesConcurrentWriters(t *testing.T) {
	hub := NewWebSocketHub("test")
	conn, server := dialTestWebSocket(t, hub)
	defer server.Close()
	defer conn.Close() //nolint:errcheck // Best-effort cleanup

	messages := startMessageReader(conn)
	waitForClientCount(t, hub, 1, time.Second)

	const rounds = 25
	const writersPerRound = 8

	expectedIDs := make(map[string]struct{}, rounds*writersPerRound)

	for round := 0; round < rounds; round++ {
		start := make(chan struct{})
		var wg sync.WaitGroup

		for writer := 0; writer < writersPerRound; writer++ {
			id := fmt.Sprintf("round-%d-writer-%d", round, writer)
			expectedIDs[id] = struct{}{}

			wg.Add(1)
			go func(messageID string) {
				defer wg.Done()
				<-start
				hub.Broadcast(map[string]any{
					"id":   messageID,
					"kind": "broadcast",
				})
			}(id)
		}

		close(start)
		wg.Wait()
	}

	received := waitForMessages(t, messages, len(expectedIDs), 5*time.Second)
	assertMessageIDs(t, received, expectedIDs)

	if got := hub.ClientCount(); got != 1 {
		t.Fatalf("ClientCount() = %d, want 1", got)
	}
}

func TestWebSocketHubBroadcastSerializesWithPingWriter(t *testing.T) {
	hub := NewWebSocketHub("test")
	hub.pingInterval = 5 * time.Millisecond

	conn, server := dialTestWebSocket(t, hub)
	defer server.Close()
	defer conn.Close() //nolint:errcheck // Best-effort cleanup

	messages := startMessageReader(conn)
	waitForClientCount(t, hub, 1, time.Second)

	const broadcasts = 30
	expectedIDs := make(map[string]struct{}, broadcasts)

	for i := 0; i < broadcasts; i++ {
		id := fmt.Sprintf("ping-race-%d", i)
		expectedIDs[id] = struct{}{}

		hub.Broadcast(map[string]any{
			"id":   id,
			"kind": "broadcast",
		})

		time.Sleep(2 * time.Millisecond)
	}

	received := waitForMessages(t, messages, len(expectedIDs), 5*time.Second)
	assertMessageIDs(t, received, expectedIDs)

	if got := hub.ClientCount(); got != 1 {
		t.Fatalf("ClientCount() = %d, want 1", got)
	}
}

func TestWebSocketHubBroadcastSerializesWithOnConnectWriter(t *testing.T) {
	hub := NewWebSocketHub("test")

	onConnectStarted := make(chan struct{})
	releaseOnConnect := make(chan struct{})

	hub.SetOnConnect(func(*WebSocketConn) any {
		close(onConnectStarted)
		<-releaseOnConnect
		return map[string]any{
			"id":   "initial-state",
			"kind": "initial",
		}
	})

	conn, server := dialTestWebSocket(t, hub)
	defer server.Close()
	defer conn.Close() //nolint:errcheck // Best-effort cleanup

	messages := startMessageReader(conn)
	waitForClientCount(t, hub, 1, time.Second)

	select {
	case <-onConnectStarted:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for onConnect callback")
	}

	const broadcasts = 20
	expectedIDs := make(map[string]struct{}, broadcasts)
	start := make(chan struct{})
	var wg sync.WaitGroup

	for i := 0; i < broadcasts; i++ {
		id := fmt.Sprintf("on-connect-race-%d", i)
		expectedIDs[id] = struct{}{}

		wg.Add(1)
		go func(messageID string) {
			defer wg.Done()
			<-start
			hub.Broadcast(map[string]any{
				"id":   messageID,
				"kind": "broadcast",
			})
		}(id)
	}

	close(start)
	close(releaseOnConnect)
	wg.Wait()

	received := waitForMessages(t, messages, broadcasts+1, 5*time.Second)
	assertMessageIDs(t, received, expectedIDs)

	if !hasMessageKind(received, "initial") {
		t.Fatal("expected initial onConnect message")
	}

	if got := hub.ClientCount(); got != 1 {
		t.Fatalf("ClientCount() = %d, want 1", got)
	}
}

func TestWebSocketHubOnConnectWriteFailureRemovesClient(t *testing.T) {
	hub := NewWebSocketHub("test")

	onConnectStarted := make(chan struct{})
	releaseOnConnect := make(chan struct{})
	disconnected := make(chan struct{}, 1)

	hub.SetOnConnect(func(*WebSocketConn) any {
		close(onConnectStarted)
		<-releaseOnConnect
		return map[string]any{
			"id":   "initial-state",
			"kind": "initial",
		}
	})

	hub.SetOnDisconnect(func(*WebSocketConn) {
		select {
		case disconnected <- struct{}{}:
		default:
		}
	})

	conn, server := dialTestWebSocket(t, hub)
	defer server.Close()
	defer conn.Close() //nolint:errcheck // Best-effort cleanup

	waitForClientCount(t, hub, 1, time.Second)

	select {
	case <-onConnectStarted:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for onConnect callback")
	}

	if err := conn.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	close(releaseOnConnect)

	waitForClientCount(t, hub, 0, time.Second)

	select {
	case <-disconnected:
	case <-time.After(time.Second):
		t.Fatal("expected onDisconnect callback after failed initial write")
	}
}

func dialTestWebSocket(t *testing.T, hub *WebSocketHub) (*websocket.Conn, *httptest.Server) {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(hub.HandleConnection))

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		server.Close()
		t.Fatalf("Dial() error = %v", err)
	}

	if resp != nil && resp.Body != nil {
		resp.Body.Close() //nolint:errcheck,gosec // Best-effort cleanup
	}

	return conn, server
}

func startMessageReader(conn *websocket.Conn) <-chan map[string]any {
	messages := make(chan map[string]any, 512)

	go func() {
		defer close(messages)

		for {
			var message map[string]any
			if err := conn.ReadJSON(&message); err != nil {
				return
			}
			messages <- message
		}
	}()

	return messages
}

func waitForClientCount(t *testing.T, hub *WebSocketHub, want int, timeout time.Duration) { //nolint:unparam // Timeout varies by caller intent
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if got := hub.ClientCount(); got == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("timed out waiting for ClientCount() == %d, got %d", want, hub.ClientCount())
}

func waitForMessages(t *testing.T, messages <-chan map[string]any, want int, timeout time.Duration) []map[string]any {
	t.Helper()

	received := make([]map[string]any, 0, want)
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	for len(received) < want {
		select {
		case message, ok := <-messages:
			if !ok {
				t.Fatalf("message stream closed after %d messages, want %d", len(received), want)
			}
			received = append(received, message)
		case <-timer.C:
			t.Fatalf("timed out waiting for %d messages, got %d", want, len(received))
		}
	}

	return received
}

func assertMessageIDs(t *testing.T, messages []map[string]any, wantIDs map[string]struct{}) {
	t.Helper()

	receivedIDs := make(map[string]struct{}, len(messages))

	for _, message := range messages {
		kind, _ := message["kind"].(string)
		if kind != "broadcast" {
			continue
		}

		id, ok := message["id"].(string)
		if !ok {
			t.Fatalf("broadcast message missing string id: %#v", message)
		}

		receivedIDs[id] = struct{}{}
	}

	if len(receivedIDs) != len(wantIDs) {
		t.Fatalf("received %d broadcast IDs, want %d", len(receivedIDs), len(wantIDs))
	}

	for id := range wantIDs {
		if _, ok := receivedIDs[id]; !ok {
			t.Fatalf("missing broadcast message id %q", id)
		}
	}
}

func hasMessageKind(messages []map[string]any, want string) bool {
	for _, message := range messages {
		kind, ok := message["kind"].(string)
		if ok && kind == want {
			return true
		}
	}

	return false
}
