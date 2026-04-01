package utils

import (
	"log/slog"
	"maps"
	"net/http"
	"slices"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// WebSocketConn wraps a WebSocket connection.
type WebSocketConn struct {
	*websocket.Conn
}

type hubClient struct {
	conn    *websocket.Conn
	wsConn  *WebSocketConn
	writeMu sync.Mutex
}

// WebSocketHub manages WebSocket connections and broadcasting.
type WebSocketHub struct {
	name         string
	clients      map[*websocket.Conn]*hubClient
	mu           sync.RWMutex
	upgrader     websocket.Upgrader
	onConnect    func(*WebSocketConn) any
	onDisconnect func(*WebSocketConn)
	pingInterval time.Duration
	pongWait     time.Duration
	writeTimeout time.Duration
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
		pingInterval: 30 * time.Second,
		pongWait:     60 * time.Second,
		writeTimeout: 10 * time.Second,
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

func (h *WebSocketHub) writeClient(client *hubClient, operation string, write func() error) error {
	client.writeMu.Lock()
	defer client.writeMu.Unlock()

	if err := client.conn.SetWriteDeadline(time.Now().Add(h.writeTimeout)); err != nil {
		slog.Warn("Failed to set WebSocket write deadline", "hub", h.name, "operation", operation, "error", err)
	}
	if err := write(); err != nil {
		return err
	}
	if err := client.conn.SetWriteDeadline(time.Time{}); err != nil {
		slog.Warn("Failed to clear WebSocket write deadline", "hub", h.name, "operation", operation, "error", err)
	}
	return nil
}

func (h *WebSocketHub) writeClientJSON(client *hubClient, operation string, data any) error {
	return h.writeClient(client, operation, func() error {
		return client.conn.WriteJSON(data)
	})
}

func (h *WebSocketHub) writeClientMessage(client *hubClient, operation string, messageType int, data []byte) error {
	return h.writeClient(client, operation, func() error {
		return client.conn.WriteMessage(messageType, data)
	})
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

	if err := client.conn.Close(); err != nil {
		slog.Warn("Failed to close WebSocket connection", "hub", h.name, "error", err)
	}

	if h.onDisconnect != nil {
		h.onDisconnect(client.wsConn)
	}

	slog.Debug("WebSocket client disconnected", "hub", h.name, "clients", clientCount)
	return clientCount
}

// HandleConnection handles a new WebSocket connection.
func (h *WebSocketHub) HandleConnection(w http.ResponseWriter, r *http.Request) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("Failed to upgrade WebSocket connection", "hub", h.name, "error", err)
		return
	}

	wsConn := &WebSocketConn{Conn: conn}
	client := &hubClient{conn: conn, wsConn: wsConn}

	h.mu.Lock()
	h.clients[conn] = client
	clientCount := len(h.clients)
	h.mu.Unlock()

	slog.Debug("WebSocket client connected", "hub", h.name, "clients", clientCount)

	if h.onConnect != nil {
		if data := h.onConnect(wsConn); data != nil {
			if err := h.writeClientJSON(client, "on_connect", data); err != nil {
				slog.Warn("Failed to send initial WebSocket data", "hub", h.name, "error", err)
				h.disconnectClient(client)
				return
			}
		}
	}

	if err := conn.SetReadDeadline(time.Now().Add(h.pongWait)); err != nil {
		slog.Warn("Failed to set read deadline", "error", err)
	}
	conn.SetPongHandler(func(string) error {
		if err := conn.SetReadDeadline(time.Now().Add(h.pongWait)); err != nil {
			slog.Warn("Failed to set read deadline", "error", err)
		}
		return nil
	})

	done := make(chan struct{})
	defer close(done)

	ticker := time.NewTicker(h.pingInterval)
	defer ticker.Stop()

	go func() {
		for {
			select {
			case <-ticker.C:
				if err := h.writeClientMessage(client, "ping", websocket.PingMessage, nil); err != nil {
					slog.Debug("WebSocket ping failed", "hub", h.name, "error", err)
					return
				}
			case <-done:
				return
			}
		}
	}()

	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
	}

	h.disconnectClient(client)
}

// Broadcast sends data to all connected clients.
func (h *WebSocketHub) Broadcast(data any) {
	h.mu.RLock()
	clients := slices.Collect(maps.Values(h.clients))
	h.mu.RUnlock()

	for _, client := range clients {
		if err := h.writeClientJSON(client, "broadcast", data); err != nil {
			slog.Warn("Failed to broadcast WebSocket message", "hub", h.name, "error", err)
			h.disconnectClient(client)
		}
	}
}

// ClientCount returns the number of connected clients.
func (h *WebSocketHub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}
