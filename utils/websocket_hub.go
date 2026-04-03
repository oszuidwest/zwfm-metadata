package utils

import (
	"encoding/json"
	"log/slog"
	"maps"
	"net/http"
	"slices"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const defaultSendBufferSize = 256

// WebSocketConn wraps a WebSocket connection.
type WebSocketConn struct {
	*websocket.Conn
}

type hubClient struct {
	conn      *websocket.Conn
	wsConn    *WebSocketConn
	send      chan []byte
	done      chan struct{}
	closeOnce sync.Once
}

// signalDone closes the done channel exactly once to signal the write pump to exit.
func (c *hubClient) signalDone() {
	c.closeOnce.Do(func() {
		close(c.done)
	})
}

// WebSocketHub manages WebSocket connections and broadcasting.
type WebSocketHub struct {
	name           string
	clients        map[*websocket.Conn]*hubClient
	mu             sync.RWMutex
	upgrader       websocket.Upgrader
	onConnect      func(*WebSocketConn) any
	onDisconnect   func(*WebSocketConn)
	pingInterval   time.Duration
	pongWait       time.Duration
	writeTimeout   time.Duration
	sendBufferSize int
}

// NewWebSocketHub returns a new WebSocketHub with the given name.
func NewWebSocketHub(name string) *WebSocketHub {
	return &WebSocketHub{
		name:    name,
		clients: make(map[*websocket.Conn]*hubClient),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
		pingInterval:   30 * time.Second,
		pongWait:       60 * time.Second,
		writeTimeout:   10 * time.Second,
		sendBufferSize: defaultSendBufferSize,
	}
}

// SetOnConnect sets the callback for new connections.
func (h *WebSocketHub) SetOnConnect(fn func(*WebSocketConn) any) {
	h.onConnect = fn
}

// SetOnDisconnect sets the callback for disconnections.
func (h *WebSocketHub) SetOnDisconnect(fn func(*WebSocketConn)) {
	h.onDisconnect = fn
}

func (h *WebSocketHub) removeClient(client *hubClient) (int, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, exists := h.clients[client.conn]; !exists {
		return len(h.clients), false
	}

	delete(h.clients, client.conn)
	return len(h.clients), true
}

func (h *WebSocketHub) disconnectClient(client *hubClient) int {
	clientCount, removed := h.removeClient(client)
	if !removed {
		return clientCount
	}

	client.signalDone()

	if err := client.conn.Close(); err != nil {
		slog.Debug("WebSocket close error", "hub", h.name, "error", err)
	}

	if h.onDisconnect != nil {
		h.onDisconnect(client.wsConn)
	}

	slog.Debug("WebSocket client disconnected", "hub", h.name, "clients", clientCount)
	return clientCount
}

// HandleConnection upgrades an HTTP connection to WebSocket and manages its lifecycle.
func (h *WebSocketHub) HandleConnection(w http.ResponseWriter, r *http.Request) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("Failed to upgrade WebSocket connection", "hub", h.name, "error", err)
		return
	}

	client := &hubClient{
		conn:   conn,
		wsConn: &WebSocketConn{Conn: conn},
		send:   make(chan []byte, h.sendBufferSize),
		done:   make(chan struct{}),
	}

	// Enqueue the onConnect payload before registering the client in the map.
	// This guarantees the initial state is first in the buffer and avoids a
	// potential deadlock: if the client were already visible to Broadcast,
	// concurrent broadcasts could fill the send buffer before writePump starts,
	// causing the blocking send below to hang forever.
	if h.onConnect != nil {
		if data := h.onConnect(client.wsConn); data != nil {
			msg, err := json.Marshal(data)
			if err != nil {
				slog.Warn("Failed to marshal initial WebSocket data", "hub", h.name, "error", err)
				conn.Close() //nolint:errcheck,gosec // Best-effort cleanup on marshal failure
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

	go h.writePump(client)
	h.readPump(client)
}

// writePump sends messages from the client's send channel to the WebSocket connection.
// It is the only goroutine that writes to the connection, guaranteeing single-writer
// safety by construction. Pings and close frames are also handled here.
func (h *WebSocketHub) writePump(client *hubClient) {
	ticker := time.NewTicker(h.pingInterval)
	defer func() {
		ticker.Stop()
		h.disconnectClient(client)
	}()

	for {
		select {
		case msg := <-client.send:
			if err := client.conn.SetWriteDeadline(time.Now().Add(h.writeTimeout)); err != nil {
				slog.Debug("WebSocket write deadline failed", "hub", h.name, "error", err)
				return
			}
			if err := client.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				slog.Debug("WebSocket write failed", "hub", h.name, "error", err)
				return
			}

			// Drain queued messages to reduce select overhead.
			for n := len(client.send); n > 0; n-- {
				if err := client.conn.SetWriteDeadline(time.Now().Add(h.writeTimeout)); err != nil {
					slog.Debug("WebSocket write deadline failed", "hub", h.name, "error", err)
					return
				}
				if err := client.conn.WriteMessage(websocket.TextMessage, <-client.send); err != nil {
					slog.Debug("WebSocket write failed", "hub", h.name, "error", err)
					return
				}
			}

		case <-client.done:
			// Best-effort close frame; errors are expected since the
			// connection may already be closed by the remote peer.
			_ = client.conn.SetWriteDeadline(time.Now().Add(h.writeTimeout))
			_ = client.conn.WriteMessage(websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			return

		case <-ticker.C:
			if err := client.conn.SetWriteDeadline(time.Now().Add(h.writeTimeout)); err != nil {
				slog.Debug("WebSocket ping deadline failed", "hub", h.name, "error", err)
				return
			}
			if err := client.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				slog.Debug("WebSocket ping failed", "hub", h.name, "error", err)
				return
			}
		}
	}
}

// readPump reads from the WebSocket connection until an error occurs. All messages
// are discarded as the hub is write-only. On exit it signals the write pump to stop.
func (h *WebSocketHub) readPump(client *hubClient) {
	defer client.signalDone()

	if err := client.conn.SetReadDeadline(time.Now().Add(h.pongWait)); err != nil {
		slog.Warn("Failed to set read deadline", "hub", h.name, "error", err)
	}
	client.conn.SetPongHandler(func(string) error {
		err := client.conn.SetReadDeadline(time.Now().Add(h.pongWait))
		if err != nil {
			slog.Debug("WebSocket pong deadline failed", "hub", h.name, "error", err)
		}
		return err
	})

	for {
		_, _, err := client.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				slog.Debug("WebSocket read error", "hub", h.name, "error", err)
			}
			return
		}
	}
}

// Broadcast serializes data once and sends it to all connected clients via their
// write buffers. Clients that cannot keep up (full send buffer) are disconnected.
func (h *WebSocketHub) Broadcast(data any) {
	msg, err := json.Marshal(data)
	if err != nil {
		slog.Warn("Failed to marshal WebSocket broadcast data", "hub", h.name, "error", err)
		return
	}

	h.mu.RLock()
	clients := slices.Collect(maps.Values(h.clients))
	h.mu.RUnlock()

	for _, client := range clients {
		select {
		case client.send <- msg:
		default:
			// Skip clients already shutting down to avoid redundant warnings.
			select {
			case <-client.done:
			default:
				slog.Warn("WebSocket client too slow, disconnecting",
					"hub", h.name, "remote_addr", client.conn.RemoteAddr())
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
