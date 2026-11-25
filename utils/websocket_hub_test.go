package utils

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

func TestWebSocketHubBroadcastSerializesConcurrentWriters(t *testing.T) {
	hub := NewWebSocketHub("test")
	conn, server := dialTestWebSocket(t, hub.HandleConnection, nil)
	defer server.Close()
	defer conn.CloseNow() //nolint:errcheck // Best-effort cleanup

	messages := startMessageReader(conn)
	waitForClientCount(t, hub, 1, time.Second)

	const rounds = 25
	const writersPerRound = 8

	expectedIDs := make(map[string]struct{}, rounds*writersPerRound)

	for round := range rounds {
		start := make(chan struct{})
		var wg sync.WaitGroup

		for writer := range writersPerRound {
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
	const broadcasts = 300

	hub := NewWebSocketHub("test")
	hub.pingInterval = 5 * time.Millisecond
	hub.sendBufferSize = broadcasts // the burst never makes the client too slow

	conn, server := dialTestWebSocket(t, hub.HandleConnection, nil)
	defer server.Close()
	defer conn.CloseNow() //nolint:errcheck // Best-effort cleanup

	messages := startMessageReader(conn)
	waitForClientCount(t, hub, 1, time.Second)

	for i := range broadcasts {
		hub.Broadcast(map[string]any{
			"id":   fmt.Sprintf("ping-race-%d", i),
			"kind": "broadcast",
		})

		if i%10 == 9 {
			time.Sleep(2 * time.Millisecond)
		}
	}

	for i, message := range waitForMessages(t, messages, broadcasts, 5*time.Second) {
		if want := fmt.Sprintf("ping-race-%d", i); message["id"] != want {
			t.Fatalf("message %d has id %v, want %s", i, message["id"], want)
		}
	}

	if got := hub.ClientCount(); got != 1 {
		t.Fatalf("ClientCount() = %d, want 1", got)
	}
}

func TestWebSocketHubBroadcastSerializesWithOnConnectWriter(t *testing.T) {
	hub := NewWebSocketHub("test")

	hub.SetOnConnect(func() any {
		return map[string]any{
			"id":   "initial-state",
			"kind": "initial",
		}
	})

	conn, server := dialTestWebSocket(t, hub.HandleConnection, nil)
	defer server.Close()
	defer conn.CloseNow() //nolint:errcheck // Best-effort cleanup

	messages := startMessageReader(conn)
	waitForClientCount(t, hub, 1, time.Second)

	const broadcasts = 20
	expectedIDs := make(map[string]struct{}, broadcasts)
	start := make(chan struct{})
	var wg sync.WaitGroup

	for i := range broadcasts {
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

func TestWebSocketHubCloseDuringOnConnectRemovesClient(t *testing.T) {
	hub := NewWebSocketHub("test")

	onConnectStarted := make(chan struct{})
	releaseOnConnect := make(chan struct{})
	disconnected := make(chan struct{}, 1)

	hub.SetOnConnect(func() any {
		close(onConnectStarted)
		<-releaseOnConnect
		return map[string]any{
			"id":   "initial-state",
			"kind": "initial",
		}
	})

	hub.SetOnDisconnect(func() {
		select {
		case disconnected <- struct{}{}:
		default:
		}
	})

	conn, server := dialTestWebSocket(t, hub.HandleConnection, nil)
	defer server.Close()
	defer conn.CloseNow() //nolint:errcheck // Best-effort cleanup

	// The client is not yet registered (onConnect is blocking), so we wait
	// for the callback to start instead of polling ClientCount.
	select {
	case <-onConnectStarted:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for onConnect callback")
	}

	// Close the client connection while onConnect is still blocked. When
	// released, HandleConnection will enqueue the message, register the
	// client, and start both pumps. readPump finds the closed connection and
	// stops writePump, whose own write may still succeed.
	if err := conn.CloseNow(); err != nil {
		t.Fatalf("CloseNow() error = %v", err)
	}

	close(releaseOnConnect)

	select {
	case <-disconnected:
	case <-time.After(time.Second):
		t.Fatal("expected onDisconnect callback for a client that left during onConnect")
	}

	waitForClientCount(t, hub, 0, time.Second)
}

func TestWebSocketHubSlowClientDisconnected(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		hub := NewWebSocketHub("test")
		hub.sendBufferSize = 1
		logs := captureLogs(t)

		var disconnects atomic.Int32
		hub.SetOnDisconnect(func() { disconnects.Add(1) })

		_, _, stall := dialStalledClient(t, hub)
		overflowSendBuffer(hub)

		// Writing works again, so only the overflow can end the connection.
		if err := stall.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
		synctest.Wait()

		if got := disconnects.Load(); got != 1 {
			t.Fatalf("onDisconnect called %d times, want 1", got)
		}
		if got := hub.ClientCount(); got != 0 {
			t.Fatalf("ClientCount() = %d, want 0", got)
		}
		if got := strings.Count(logs.String(), "too slow"); got != 1 {
			t.Fatalf("logged %d slow client warnings, want 1", got)
		}
		if !strings.Contains(logs.String(), "remote_addr=192.0.2.1:") {
			t.Fatalf("log = %s, want the client address", logs)
		}
	})
}

func TestWebSocketHubClientInitiatedClose(t *testing.T) {
	hub := NewWebSocketHub("test")

	disconnected := make(chan struct{}, 1)
	hub.SetOnDisconnect(func() {
		select {
		case disconnected <- struct{}{}:
		default:
		}
	})

	conn, server := dialTestWebSocket(t, hub.HandleConnection, nil)
	defer server.Close()

	waitForClientCount(t, hub, 1, time.Second)

	// Client-initiated close: readPump detects the closed connection,
	// signals done, writePump exits via its deferred disconnectClient.
	conn.Close(websocket.StatusNormalClosure, "") //nolint:errcheck,gosec // Best-effort cleanup

	select {
	case <-disconnected:
	case <-time.After(2 * time.Second):
		t.Fatal("expected onDisconnect after client-initiated close")
	}

	waitForClientCount(t, hub, 0, time.Second)
}

func TestWebSocketHubBroadcastMarshalFailure(t *testing.T) {
	hub := NewWebSocketHub("test")

	conn, server := dialTestWebSocket(t, hub.HandleConnection, nil)
	defer server.Close()
	defer conn.CloseNow() //nolint:errcheck // Best-effort cleanup

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

	hub.SetOnConnect(func() any {
		return make(chan int)
	})

	conn, server := dialTestWebSocket(t, hub.HandleConnection, nil)
	defer server.Close()
	defer conn.CloseNow() //nolint:errcheck // Best-effort cleanup

	// The marshal failure in HandleConnection closes the connection
	// before registration, so the client never appears in the map.
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	if _, _, err := conn.Read(ctx); err == nil || ctx.Err() != nil {
		t.Fatalf("Read() error = %v, want a closed connection", err)
	}

	if got := hub.ClientCount(); got != 0 {
		t.Fatalf("ClientCount() = %d after onConnect marshal failure, want 0", got)
	}
}

func TestWebSocketHubOnDisconnectCalledOnce(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		hub := NewWebSocketHub("test")
		hub.sendBufferSize = 1

		var disconnects atomic.Int32
		hub.SetOnDisconnect(func() { disconnects.Add(1) })

		// A slow client that also goes away ends the connection in three ways:
		// Broadcast and readPump signal done, and the stalled write times out.
		conn, client, _ := dialStalledClient(t, hub)
		overflowSendBuffer(hub)
		if err := conn.CloseNow(); err != nil {
			t.Fatalf("CloseNow() error = %v", err)
		}
		time.Sleep(hub.writeTimeout)
		synctest.Wait()

		if got := hub.ClientCount(); got != 0 {
			t.Fatalf("ClientCount() = %d, want 0", got)
		}

		// A second cleanup of the same client must not report it again.
		hub.disconnectClient(client)
		if got := disconnects.Load(); got != 1 {
			t.Fatalf("onDisconnect called %d times, want 1", got)
		}
	})
}

func TestWebSocketHubSendsHeartbeatWhenIdle(t *testing.T) {
	hub := NewWebSocketHub("test")
	hub.SetHeartbeat(20*time.Millisecond, []byte(`{"kind":"heartbeat"}`))

	conn, server := dialTestWebSocket(t, hub.HandleConnection, nil)
	defer server.Close()
	defer conn.CloseNow() //nolint:errcheck // Best-effort cleanup

	messages := startMessageReader(conn)
	for _, message := range waitForMessages(t, messages, 2, time.Second) {
		if message["kind"] != "heartbeat" {
			t.Fatalf("message = %#v, want a heartbeat", message)
		}
	}
}

func TestWebSocketHubBroadcastPostponesHeartbeat(t *testing.T) {
	hub := NewWebSocketHub("test")
	hub.SetHeartbeat(250*time.Millisecond, []byte(`{"kind":"heartbeat"}`))

	conn, server := dialTestWebSocket(t, hub.HandleConnection, nil)
	defer server.Close()
	defer conn.CloseNow() //nolint:errcheck // Best-effort cleanup

	messages := startMessageReader(conn)
	waitForClientCount(t, hub, 1, time.Second)

	const broadcasts = 24
	for i := range broadcasts {
		hub.Broadcast(map[string]any{"id": fmt.Sprintf("busy-%d", i), "kind": "broadcast"})
		time.Sleep(25 * time.Millisecond)
	}

	received := waitForMessages(t, messages, broadcasts, time.Second)
	if hasMessageKind(received, "heartbeat") {
		t.Fatal("heartbeat sent while broadcasts kept the connection busy")
	}

	if last := waitForMessages(t, messages, 1, time.Second); last[0]["kind"] != "heartbeat" {
		t.Fatalf("message after the broadcasts = %#v, want a heartbeat", last[0])
	}
}

func TestWebSocketHubDropsPeerThatStopsAnsweringPings(t *testing.T) {
	hub := NewWebSocketHub("test")
	hub.pingInterval = 20 * time.Millisecond
	hub.pongWait = 50 * time.Millisecond

	disconnected := make(chan struct{}, 1)
	hub.SetOnDisconnect(func() { disconnected <- struct{}{} })

	// A client that does not read does not answer pings either.
	conn, server := dialTestWebSocket(t, hub.HandleConnection, nil)
	defer server.Close()
	defer conn.CloseNow() //nolint:errcheck // Best-effort cleanup

	select {
	case <-disconnected:
	case <-time.After(time.Second):
		t.Fatal("expected a peer that does not answer pings to be disconnected")
	}
}

func TestWebSocketHubDropsPeerInEndlessFrame(t *testing.T) {
	hub := NewWebSocketHub("test")
	hub.pingInterval = 20 * time.Millisecond
	hub.pongWait = 50 * time.Millisecond

	handlerDone := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(handlerDone)
		hub.HandleConnection(w, r)
	}))
	defer server.Close()

	conn, err := net.Dial("tcp", server.Listener.Addr().String())
	if err != nil {
		t.Fatalf("Dial() error = %v", err)
	}
	defer conn.Close() //nolint:errcheck // Best-effort cleanup

	// A binary frame header that announces 1 TiB, followed by payload without end.
	// The peer cannot answer a ping or a close frame in the middle of a frame.
	request := "GET / HTTP/1.1\r\nHost: test\r\nConnection: Upgrade\r\nUpgrade: websocket\r\n" +
		"Sec-WebSocket-Version: 13\r\nSec-WebSocket-Key: AAAAAAAAAAAAAAAAAAAAAA==\r\n\r\n"
	header := "\x82\xff\x00\x00\x01\x00\x00\x00\x00\x00\x00\x00\x00\x00"
	if _, err := conn.Write([]byte(request + header)); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	go func() {
		payload := make([]byte, 1024)
		for {
			if _, err := conn.Write(payload); err != nil {
				return
			}
			time.Sleep(time.Millisecond)
		}
	}()

	select {
	case <-handlerDone:
	case <-time.After(2 * time.Second):
		t.Fatal("expected a peer in an endless frame to be disconnected")
	}
}

func TestWebSocketHubPendingPongDoesNotHoldUpMessages(t *testing.T) {
	hub := NewWebSocketHub("test")
	hub.pingInterval = 20 * time.Millisecond

	pinged := make(chan struct{}, 1)
	conn, server := dialTestWebSocket(t, hub.HandleConnection, &websocket.DialOptions{
		OnPingReceived: func(context.Context, []byte) bool {
			select {
			case pinged <- struct{}{}:
			default:
			}
			return false
		},
	})
	defer server.Close()
	defer conn.CloseNow() //nolint:errcheck // Best-effort cleanup

	messages := startMessageReader(conn)
	select {
	case <-pinged:
	case <-time.After(time.Second):
		t.Fatal("expected a ping")
	}

	hub.Broadcast(map[string]any{"kind": "broadcast"})
	waitForMessages(t, messages, 1, time.Second)
}

func TestWebSocketHubDropsClientOnWriteTimeout(t *testing.T) {
	hub := NewWebSocketHub("test")
	hub.writeTimeout = 50 * time.Millisecond

	conn, server := dialTestWebSocket(t, hub.HandleConnection, nil)
	defer server.Close()
	defer conn.CloseNow() //nolint:errcheck // Best-effort cleanup

	waitForClientCount(t, hub, 1, time.Second)

	// Too large for the socket buffers, so the write stalls on a client that does not read.
	hub.BroadcastMessage(make([]byte, 32<<20))
	waitForClientCount(t, hub, 0, 2*time.Second)
}

func TestWebSocketHubDiscardsClientMessages(t *testing.T) {
	hub := NewWebSocketHub("test")

	conn, server := dialTestWebSocket(t, hub.HandleConnection, nil)
	defer server.Close()
	defer conn.CloseNow() //nolint:errcheck // Best-effort cleanup

	conn.CloseRead(t.Context())
	if err := conn.Write(t.Context(), websocket.MessageBinary, make([]byte, 1<<20)); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	// The pong proves that the hub has read past the message.
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	if err := conn.Ping(ctx); err != nil {
		t.Fatalf("Ping() error = %v", err)
	}
}

func TestWebSocketHubAcceptsAnyOrigin(t *testing.T) {
	hub := NewWebSocketHub("test")

	conn, server := dialTestWebSocket(t, hub.HandleConnection, &websocket.DialOptions{
		HTTPHeader: http.Header{"Origin": {"https://other.example"}},
	})
	conn.CloseNow() //nolint:errcheck,gosec // Best-effort cleanup
	server.Close()
}

func TestWebSocketHubLimitsUpgradeErrorLog(t *testing.T) {
	hub := NewWebSocketHub("test")
	logs := captureLogs(t)

	r := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	r.Header.Set("Connection", strings.Repeat("x", 10000))
	hub.HandleConnection(httptest.NewRecorder(), r)

	if !strings.Contains(logs.String(), "Failed to upgrade") || logs.Len() > 1000 {
		t.Fatalf("log has %d bytes, want one short upgrade error", logs.Len())
	}
}

// captureLogs redirects the default logger to a buffer for the duration of the test.
func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()

	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })

	return &logs
}

func dialTestWebSocket(t *testing.T, handler http.HandlerFunc, opts *websocket.DialOptions) (*websocket.Conn, *httptest.Server) {
	t.Helper()

	server := httptest.NewServer(handler)

	conn, resp, err := websocket.Dial(t.Context(), server.URL, opts)
	if err != nil {
		server.Close()
		t.Fatalf("Dial() error = %v", err)
	}

	if resp != nil && resp.Body != nil {
		resp.Body.Close() //nolint:errcheck,gosec // Best-effort cleanup
	}

	return conn, server
}

// dialStalledClient connects a client to which the hub cannot write until stall
// is closed. It must run in a synctest bubble.
func dialStalledClient(t *testing.T, hub *WebSocketHub) (conn *websocket.Conn, client *hubClient, stall io.Closer) {
	t.Helper()

	server := httptest.NewTestServer(t, http.HandlerFunc(hub.HandleConnection))
	conn, resp, err := websocket.Dial(t.Context(), server.URL, &websocket.DialOptions{HTTPClient: server.Client()})
	if err != nil {
		t.Fatalf("Dial() error = %v", err)
	}
	t.Cleanup(func() { conn.CloseNow() }) //nolint:errcheck,gosec // Best-effort cleanup
	if resp != nil && resp.Body != nil {
		resp.Body.Close() //nolint:errcheck,gosec // Best-effort cleanup
	}

	synctest.Wait()
	hub.mu.RLock()
	for _, c := range hub.clients {
		client = c
	}
	hub.mu.RUnlock()
	if client == nil {
		t.Fatal("client is not registered")
	}

	// An open message writer holds up every other message on the connection.
	stall, err = client.conn.Writer(t.Context(), websocket.MessageText)
	if err != nil {
		t.Fatalf("Writer() error = %v", err)
	}
	return conn, client, stall
}

// overflowSendBuffer broadcasts until a stalled client with a send buffer of one
// has a message being written, a message queued and a message that does not fit.
func overflowSendBuffer(hub *WebSocketHub) {
	for i := range 3 {
		hub.Broadcast(map[string]any{"i": i})
		synctest.Wait()
	}
}

func startMessageReader(conn *websocket.Conn) <-chan map[string]any {
	messages := make(chan map[string]any, 512)

	go func() {
		defer close(messages)

		for {
			var message map[string]any
			if err := wsjson.Read(context.Background(), conn, &message); err != nil {
				return
			}
			messages <- message
		}
	}()

	return messages
}

func waitForClientCount(t *testing.T, hub *WebSocketHub, want int, timeout time.Duration) {
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
