package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
)

const defaultSendBufferSize = 256

type hubClient struct {
	conn       *websocket.Conn
	remoteAddr string
	send       chan []byte
	done       <-chan struct{}
	signalDone context.CancelFunc
}

// WebSocketHub manages WebSocket connections and broadcasting.
type WebSocketHub struct {
	name           string
	clients        map[*websocket.Conn]*hubClient
	mu             sync.RWMutex
	onConnect      func() any
	heartbeat      []byte
	heartbeatAfter time.Duration
	pingInterval   time.Duration
	pongWait       time.Duration
	writeTimeout   time.Duration
	sendBufferSize int
}

// NewWebSocketHub returns a new WebSocketHub with the given name.
func NewWebSocketHub(name string) *WebSocketHub {
	return &WebSocketHub{
		name:           name,
		clients:        make(map[*websocket.Conn]*hubClient),
		pingInterval:   30 * time.Second,
		pongWait:       60 * time.Second,
		writeTimeout:   10 * time.Second,
		sendBufferSize: defaultSendBufferSize,
	}
}

// SetOnConnect sets the callback that provides the initial payload for new connections.
func (h *WebSocketHub) SetOnConnect(fn func() any) {
	h.onConnect = fn
}

// SetHeartbeat sends msg to a client after it received nothing for the given
// interval. Unlike ping frames, browser clients can see it and detect a dead
// connection. A zero interval disables the heartbeat.
func (h *WebSocketHub) SetHeartbeat(interval time.Duration, msg []byte) {
	h.heartbeatAfter = interval
	h.heartbeat = msg
}

func (h *WebSocketHub) disconnectClient(client *hubClient) {
	h.mu.Lock()
	delete(h.clients, client.conn)
	clientCount := len(h.clients)
	h.mu.Unlock()

	// CloseNow, because Close waits without a limit for a peer that keeps sending.
	client.conn.CloseNow() //nolint:errcheck,gosec // The peer may have closed it already

	slog.Debug("WebSocket client disconnected", "hub", h.name, "clients", clientCount)
}

// HandleConnection upgrades an HTTP connection to WebSocket and manages its lifecycle.
func (h *WebSocketHub) HandleConnection(w http.ResponseWriter, r *http.Request) {
	var pongDeadline *time.Timer // set before readPump, which handles the pongs
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true, // any origin may connect
		OnPongReceived:     func(context.Context, []byte) { pongDeadline.Reset(h.pongWait) },
	})
	if err != nil {
		// The error repeats request headers, which can be very long.
		slog.Error("Failed to upgrade WebSocket connection", "hub", h.name, "error", fmt.Sprintf("%.200s", err))
		return
	}

	// The deferred cancel runs when readPump returns and tells writePump to stop.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client := &hubClient{
		conn:       conn,
		remoteAddr: r.RemoteAddr,
		send:       make(chan []byte, h.sendBufferSize),
		done:       ctx.Done(),
		signalDone: cancel,
	}

	// Enqueue the onConnect payload before registering the client in the map.
	// This guarantees the initial state is first in the buffer and avoids a
	// potential deadlock: if the client were already visible to Broadcast,
	// concurrent broadcasts could fill the send buffer before writePump starts,
	// causing the blocking send below to hang forever.
	if h.onConnect != nil {
		if data := h.onConnect(); data != nil {
			msg, err := json.Marshal(data)
			if err != nil {
				slog.Warn("Failed to marshal initial WebSocket data", "hub", h.name, "error", err)
				conn.CloseNow() //nolint:errcheck,gosec // Best-effort cleanup on marshal failure
				return
			}
			client.send <- msg
		}
	}

	h.mu.Lock()
	h.clients[conn] = client
	clientCount := len(h.clients)
	h.mu.Unlock()

	slog.Debug("WebSocket client connected", "hub", h.name, "clients", clientCount)

	pongDeadline = time.AfterFunc(h.pongWait, client.signalDone)
	defer pongDeadline.Stop()

	go h.writePump(client)
	h.readPump(client)
}

// writePump sends messages from the client's send channel to the WebSocket connection.
// It is the only goroutine that writes messages, which keeps them in order. Pings are
// started and the connection is closed here too.
func (h *WebSocketHub) writePump(client *hubClient) {
	pings := time.NewTicker(h.pingInterval)
	defer pings.Stop()
	defer h.disconnectClient(client)

	// Without a heartbeat the nil channel never fires.
	var idle *time.Timer
	var heartbeats <-chan time.Time
	if h.heartbeatAfter > 0 {
		idle = time.NewTimer(h.heartbeatAfter)
		defer idle.Stop()
		heartbeats = idle.C
	}

	for {
		select {
		case msg := <-client.send:
			if !h.writeText(client, msg) {
				return
			}
			if idle != nil {
				idle.Reset(h.heartbeatAfter)
			}

		case <-heartbeats:
			if !h.writeText(client, h.heartbeat) {
				return
			}
			idle.Reset(h.heartbeatAfter)

		case <-client.done:
			return

		case <-pings.C:
			// Ping waits for the pong, which must not hold up messages.
			go h.ping(client)
		}
	}
}

// ping sends a ping and waits a moment for the pong. It does not drop the client, because
// a later pong still counts. The pong deadline drops a client that stays silent.
func (h *WebSocketHub) ping(client *hubClient) {
	ctx, cancel := context.WithTimeout(context.Background(), h.writeTimeout)
	defer cancel()

	if err := client.conn.Ping(ctx); err != nil {
		slog.Debug("WebSocket ping failed", "hub", h.name, "error", err)
	}
}

// writeText writes one text frame and reports whether the connection is still usable.
func (h *WebSocketHub) writeText(client *hubClient, msg []byte) bool {
	ctx, cancel := context.WithTimeout(context.Background(), h.writeTimeout)
	defer cancel()

	if err := client.conn.Write(ctx, websocket.MessageText, msg); err != nil {
		slog.Debug("WebSocket write failed", "hub", h.name, "error", err)
		return false
	}
	return true
}

// readPump reads from the WebSocket connection until an error occurs. All messages
// are discarded as the hub is write-only.
func (h *WebSocketHub) readPump(client *hubClient) {
	// Messages are discarded without buffering, so their size does not matter.
	client.conn.SetReadLimit(-1)

	for {
		_, r, err := client.conn.Reader(context.Background())
		if err == nil {
			_, err = io.Copy(io.Discard, r)
		}
		if err != nil {
			status := websocket.CloseStatus(err)
			if status != websocket.StatusNormalClosure && status != websocket.StatusGoingAway {
				slog.Debug("WebSocket read error", "hub", h.name, "error", err)
			}
			return
		}
	}
}

// Broadcast serializes data once and sends it to all connected clients.
func (h *WebSocketHub) Broadcast(data any) {
	msg, err := json.Marshal(data)
	if err != nil {
		slog.Warn("Failed to marshal WebSocket broadcast data", "hub", h.name, "error", err)
		return
	}
	h.BroadcastMessage(msg)
}

// BroadcastMessage queues an already serialized message on every client's write
// buffer. Clients that cannot keep up (full send buffer) are disconnected.
func (h *WebSocketHub) BroadcastMessage(msg []byte) {
	// Sends are non-blocking and disconnects happen in writePump, so holding the
	// read lock for the whole loop cannot deadlock.
	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, client := range h.clients {
		select {
		case client.send <- msg:
		default:
			// Skip clients already shutting down to avoid redundant warnings.
			select {
			case <-client.done:
			default:
				slog.Warn("WebSocket client too slow, disconnecting",
					"hub", h.name, "remote_addr", client.remoteAddr)
				client.signalDone()
			}
		}
	}
}

// ClientCount returns the number of connected clients.
func (h *WebSocketHub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}
