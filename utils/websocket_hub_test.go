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

	hub.SetOnConnect(func(*WebSocketConn) any {
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
	wg.Wait()

	received := waitForMessages(t, messages, broadcasts+1, 5*time.Second)
	assertMessageIDs(t, received, expectedIDs)

	if !hasMessageKind(received, "initial") {
		t.Fatal("expected initial onConnect message")
	}

	// The onConnect message must be first: it is enqueued before the client
	// becomes visible to Broadcast, so no broadcast can precede it.
	if kind, _ := received[0]["kind"].(string); kind != "initial" {
		t.Fatal("expected onConnect message to be delivered first")
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

	// The client is not yet registered (onConnect is blocking), so we wait
	// for the callback to start instead of polling ClientCount.
	select {
	case <-onConnectStarted:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for onConnect callback")
	}

	// Close the client connection while onConnect is still blocked. When
	// released, HandleConnection will enqueue the message, register the
	// client, and start writePump -- which will fail on the closed connection.
	if err := conn.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	close(releaseOnConnect)

	select {
	case <-disconnected:
	case <-time.After(time.Second):
		t.Fatal("expected onDisconnect callback after failed initial write")
	}

	waitForClientCount(t, hub, 0, time.Second)
}

func TestWebSocketHubSlowClientDisconnected(t *testing.T) {
	hub := NewWebSocketHub("test")
	hub.sendBufferSize = 2
	hub.writeTimeout = 50 * time.Millisecond

	disconnected := make(chan struct{}, 1)
	hub.SetOnDisconnect(func(*WebSocketConn) {
		select {
		case disconnected <- struct{}{}:
		default:
		}
	})

	// Connect but intentionally do not read messages. The small send buffer
	// will fill up and Broadcast will detect the slow client.
	conn, server := dialTestWebSocket(t, hub)
	defer server.Close()
	defer conn.Close() //nolint:errcheck // Best-effort cleanup

	waitForClientCount(t, hub, 1, time.Second)

	for i := 0; i < 100; i++ {
		hub.Broadcast(map[string]any{"i": i})
	}

	select {
	case <-disconnected:
	case <-time.After(5 * time.Second):
		t.Fatal("expected slow client to be disconnected")
	}

	waitForClientCount(t, hub, 0, 2*time.Second)
}

func TestWebSocketHubClientInitiatedClose(t *testing.T) {
	hub := NewWebSocketHub("test")

	disconnected := make(chan struct{}, 1)
	hub.SetOnDisconnect(func(*WebSocketConn) {
		select {
		case disconnected <- struct{}{}:
		default:
		}
	})

	conn, server := dialTestWebSocket(t, hub)
	defer server.Close()

	waitForClientCount(t, hub, 1, time.Second)

	// Client-initiated close: readPump detects the closed connection,
	// signals done, writePump sends a close frame and exits via its
	// deferred disconnectClient.
	conn.WriteMessage( //nolint:errcheck,gosec // Best-effort close frame
		websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
	)
	conn.Close() //nolint:errcheck,gosec // Best-effort cleanup

	select {
	case <-disconnected:
	case <-time.After(2 * time.Second):
		t.Fatal("expected onDisconnect after client-initiated close")
	}

	waitForClientCount(t, hub, 0, time.Second)
}

func TestWebSocketHubBroadcastMarshalFailure(t *testing.T) {
	hub := NewWebSocketHub("test")

	conn, server := dialTestWebSocket(t, hub)
	defer server.Close()
	defer conn.Close() //nolint:errcheck // Best-effort cleanup

	waitForClientCount(t, hub, 1, time.Second)

	// Channels cannot be marshaled to JSON. Broadcast should return
	// without panicking or disconnecting any client.
	hub.Broadcast(make(chan int))

	if got := hub.ClientCount(); got != 1 {
		t.Fatalf("ClientCount() = %d after marshal failure, want 1", got)
	}
}

func TestWebSocketHubOnConnectMarshalFailureRemovesClient(t *testing.T) {
	hub := NewWebSocketHub("test")

	// Return an un-marshalable value from onConnect.
	hub.SetOnConnect(func(*WebSocketConn) any {
		return make(chan int)
	})

	conn, server := dialTestWebSocket(t, hub)
	defer server.Close()
	defer conn.Close() //nolint:errcheck // Best-effort cleanup

	// The marshal failure in HandleConnection closes the connection
	// before registration, so the client never appears in the map.
	time.Sleep(100 * time.Millisecond)

	if got := hub.ClientCount(); got != 0 {
		t.Fatalf("ClientCount() = %d after onConnect marshal failure, want 0", got)
	}
}

func TestWebSocketHubOnDisconnectCalledOnce(t *testing.T) {
	hub := NewWebSocketHub("test")
	hub.sendBufferSize = 1
	hub.writeTimeout = 50 * time.Millisecond

	var count sync.WaitGroup
	count.Add(1)

	var calls int32
	var mu sync.Mutex
	hub.SetOnDisconnect(func(*WebSocketConn) {
		mu.Lock()
		calls++
		mu.Unlock()
		count.Done()
	})

	// Connect but do not read, so the send buffer fills quickly.
	conn, server := dialTestWebSocket(t, hub)
	defer server.Close()
	defer conn.Close() //nolint:errcheck // Best-effort cleanup

	waitForClientCount(t, hub, 1, time.Second)

	// Flood broadcasts to trigger signalDone from Broadcast (slow client)
	// while writePump may also exit due to a write error. Both paths
	// converge on disconnectClient, which must call onDisconnect exactly once.
	for i := 0; i < 50; i++ {
		hub.Broadcast(map[string]any{"i": i})
	}

	count.Wait()
	waitForClientCount(t, hub, 0, 2*time.Second)

	// Give any stray goroutines a moment to settle.
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if calls != 1 {
		t.Fatalf("onDisconnect called %d times, want 1", calls)
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
