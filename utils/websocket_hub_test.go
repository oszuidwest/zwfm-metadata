package utils

import (
	"bufio"
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

func TestWebSocketHubOnConnectWriteFailureRemovesClient(t *testing.T) {
	hub := NewWebSocketHub("test")

	onConnectStarted := make(chan struct{})
	releaseOnConnect := make(chan struct{})
	handler, handlerDone := trackHandler(hub)

	hub.SetOnConnect(func() any {
		close(onConnectStarted)
		<-releaseOnConnect
		return map[string]any{
			"id":   "initial-state",
			"kind": "initial",
		}
	})

	conn, server := dialTestWebSocket(t, handler, nil)
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
	// client, and start writePump -- which will fail on the closed connection.
	if err := conn.CloseNow(); err != nil {
		t.Fatalf("CloseNow() error = %v", err)
	}

	close(releaseOnConnect)

	// The count is also 0 before the client is registered, so wait until it has been.
	waitForHandler(t, handlerDone, time.Second)

	waitForClientCount(t, hub, 0, time.Second)
}

func TestWebSocketHubSlowClientDisconnected(t *testing.T) {
	hub := NewWebSocketHub("test")
	hub.sendBufferSize = 2
	hub.writeTimeout = 50 * time.Millisecond
	logs := captureLogs(t)
	handler, handlerDone := trackHandler(hub)

	// Connect but intentionally do not read messages. The small send buffer
	// will fill up and Broadcast will detect the slow client.
	conn, server := dialTestWebSocket(t, handler, nil)
	defer server.Close()
	defer conn.CloseNow() //nolint:errcheck // Best-effort cleanup

	waitForClientCount(t, hub, 1, time.Second)

	for i := range 100 {
		hub.Broadcast(map[string]any{"i": i})
	}

	waitForHandler(t, handlerDone, 5*time.Second)
	waitForClientCount(t, hub, 0, 2*time.Second)

	if got := strings.Count(logs.String(), "too slow"); got != 1 {
		t.Fatalf("logged %d slow client warnings, want 1", got)
	}
	if !strings.Contains(logs.String(), "remote_addr=127.0.0.1:") {
		t.Fatalf("log = %s, want the client address", logs)
	}
}

func TestWebSocketHubClientInitiatedClose(t *testing.T) {
	hub := NewWebSocketHub("test")

	conn, server := dialTestWebSocket(t, hub.HandleConnection, nil)
	defer server.Close()

	waitForClientCount(t, hub, 1, time.Second)

	// Client-initiated close: readPump detects the closed connection and
	// returns, which makes writePump exit via its deferred disconnectClient.
	conn.Close(websocket.StatusNormalClosure, "") //nolint:errcheck,gosec // Best-effort cleanup

	waitForClientCount(t, hub, 0, 2*time.Second)
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

	// A client that does not read does not answer pings either.
	handler, handlerDone := trackHandler(hub)
	conn, server := dialTestWebSocket(t, handler, nil)
	defer server.Close()
	defer conn.CloseNow() //nolint:errcheck // Best-effort cleanup

	waitForHandler(t, handlerDone, time.Second)
	waitForClientCount(t, hub, 0, time.Second)
}

// pipeResponse hands one end of a pipe to the handler as its connection.
type pipeResponse struct {
	*httptest.ResponseRecorder
	conn net.Conn
}

func (p pipeResponse) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return p.conn, bufio.NewReadWriter(bufio.NewReader(p.conn), bufio.NewWriter(p.conn)), nil
}

func TestWebSocketHubCountsDeadlineFromLastPong(t *testing.T) {
	tests := []struct {
		name  string
		pongs []time.Duration
		want  time.Duration
	}{
		{"no pong", nil, 60 * time.Second},
		{"pong 10 seconds after the ping, then silence", []time.Duration{40 * time.Second}, 100 * time.Second},
		{"pongs 29 and 58 seconds after the ping", []time.Duration{59 * time.Second, 118 * time.Second}, 178 * time.Second},
		{"unsolicited pong before the first ping", []time.Duration{time.Second}, 61 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				hub := NewWebSocketHub("test")
				server, client := net.Pipe()
				defer client.Close() //nolint:errcheck // Best-effort cleanup

				r := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
				r.Header.Set("Connection", "Upgrade")
				r.Header.Set("Upgrade", "websocket")
				r.Header.Set("Sec-WebSocket-Version", "13")
				r.Header.Set("Sec-WebSocket-Key", "AAAAAAAAAAAAAAAAAAAAAA==")

				start := time.Now()
				go io.Copy(io.Discard, client) //nolint:errcheck // Ends when the hub closes the pipe
				for i, at := range tt.pongs {
					time.AfterFunc(at, func() {
						client.Write([]byte{0x8a, 0x81, 0, 0, 0, 0, byte('1' + i)}) //nolint:errcheck,gosec // The hub may have closed the pipe
					})
				}

				hub.HandleConnection(pipeResponse{httptest.NewRecorder(), server}, r)
				if got := time.Since(start); got != tt.want {
					t.Fatalf("dropped after %v, want %v", got, tt.want)
				}
			})
		})
	}
}

func TestWebSocketHubDropsPeerInEndlessFrame(t *testing.T) {
	for name, keepsSending := range map[string]bool{"keeps sending": true, "stops sending": false} {
		t.Run(name, func(t *testing.T) {
			hub := NewWebSocketHub("test")
			hub.pingInterval = 20 * time.Millisecond
			hub.pongWait = 50 * time.Millisecond

			handler, handlerDone := trackHandler(hub)
			server := httptest.NewServer(handler)
			defer server.Close()

			conn, err := net.Dial("tcp", server.Listener.Addr().String())
			if err != nil {
				t.Fatalf("Dial() error = %v", err)
			}
			defer conn.Close() //nolint:errcheck // Best-effort cleanup

			// A binary frame header that announces 1 TiB. The peer cannot answer a ping
			// or a close frame in the middle of a frame.
			request := "GET / HTTP/1.1\r\nHost: test\r\nConnection: Upgrade\r\nUpgrade: websocket\r\n" +
				"Sec-WebSocket-Version: 13\r\nSec-WebSocket-Key: AAAAAAAAAAAAAAAAAAAAAA==\r\n\r\n"
			header := "\x82\xff\x00\x00\x01\x00\x00\x00\x00\x00\x00\x00\x00\x00"
			if _, err := conn.Write([]byte(request + header)); err != nil {
				t.Fatalf("Write() error = %v", err)
			}
			go func() {
				payload := make([]byte, 1024)
				for keepsSending {
					if _, err := conn.Write(payload); err != nil {
						return
					}
					time.Sleep(time.Millisecond)
				}
			}()

			waitForHandler(t, handlerDone, 2*time.Second)
		})
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

// trackHandler returns the hub's handler and a channel that closes when the handler
// returns for its one connection, which it does once readPump has ended.
func trackHandler(hub *WebSocketHub) (handler http.HandlerFunc, returned <-chan struct{}) {
	done := make(chan struct{})

	return func(w http.ResponseWriter, r *http.Request) {
		defer close(done)
		hub.HandleConnection(w, r)
	}, done
}

func waitForHandler(t *testing.T, done <-chan struct{}, timeout time.Duration) {
	t.Helper()

	select {
	case <-done:
	case <-time.After(timeout):
		t.Fatal("timed out waiting for HandleConnection to return")
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
