package utils

import (
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// WebSocketConn wraps a WebSocket connection
type WebSocketConn struct {
	*websocket.Conn
}

// WebSocketHub manages WebSocket connections and broadcasting
type WebSocketHub struct {
	name         string
	clients      map[*websocket.Conn]bool
	mu           sync.RWMutex
	upgrader     websocket.Upgrader
	onConnect    func(*WebSocketConn) interface{}
	onDisconnect func(*WebSocketConn)
	pingInterval time.Duration
	pongWait     time.Duration
	writeTimeout time.Duration
}

// NewWebSocketHub creates a new WebSocket hub
func NewWebSocketHub(name string) *WebSocketHub {
	return &WebSocketHub{
		name:    name,
		clients: make(map[*websocket.Conn]bool),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins in development
			},
		},
		pingInterval: 30 * time.Second,
		pongWait:     60 * time.Second,
		writeTimeout: 10 * time.Second,
	}
}

// SetOnConnect sets the callback for new connections
func (h *WebSocketHub) SetOnConnect(fn func(*WebSocketConn) interface{}) {
	h.onConnect = fn
}

// SetOnDisconnect sets the callback for disconnections
func (h *WebSocketHub) SetOnDisconnect(fn func(*WebSocketConn)) {
	h.onDisconnect = fn
}

// HandleConnection handles a new WebSocket connection
func (h *WebSocketHub) HandleConnection(w http.ResponseWriter, r *http.Request) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("Failed to upgrade WebSocket connection", "hub", h.name, "error", err)
		return
	}

	// Add client
	h.mu.Lock()
	h.clients[conn] = true
	clientCount := len(h.clients)
	h.mu.Unlock()

	slog.Debug("WebSocket client connected", "hub", h.name, "clients", clientCount)

	// Wrap connection
	wsConn := &WebSocketConn{Conn: conn}

	// Send initial data if callback is set
	if h.onConnect != nil {
		if data := h.onConnect(wsConn); data != nil {
			if err := conn.SetWriteDeadline(time.Now().Add(h.writeTimeout)); err != nil {
				slog.Warn("Failed to set write deadline", "error", err)
			}
			if err := conn.WriteJSON(data); err != nil {
				slog.Debug("Failed to send initial data", "hub", h.name, "error", err)
			}
			if err := conn.SetWriteDeadline(time.Time{}); err != nil {
				slog.Warn("Failed to clear write deadline", "error", err)
			}
		}
	}

	// Setup ping/pong
	if err := conn.SetReadDeadline(time.Now().Add(h.pongWait)); err != nil {
		slog.Warn("Failed to set read deadline", "error", err)
	}
	conn.SetPongHandler(func(string) error {
		if err := conn.SetReadDeadline(time.Now().Add(h.pongWait)); err != nil {
			slog.Warn("Failed to set read deadline", "error", err)
		}
		return nil
	})

	// Create done channel for cleanup
	done := make(chan struct{})
	defer close(done)

	// Start ping ticker
	ticker := time.NewTicker(h.pingInterval)
	defer ticker.Stop()

	// Ping sender goroutine
	go func() {
		for {
			select {
			case <-ticker.C:
				if err := conn.SetWriteDeadline(time.Now().Add(h.writeTimeout)); err != nil {
					return
				}
				if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					return
				}
				if err := conn.SetWriteDeadline(time.Time{}); err != nil {
					return
				}
			case <-done:
				return
			}
		}
	}()

	// Read messages (mainly for detecting disconnection)
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
	}

	// Cleanup on disconnect
	// Close connection first to ensure no more writes can happen
	if err := conn.Close(); err != nil {
		slog.Warn("Failed to close connection", "error", err)
	}

	// Then remove from clients map
	h.mu.Lock()
	delete(h.clients, conn)
	clientCount = len(h.clients)
	h.mu.Unlock()

	if h.onDisconnect != nil {
		h.onDisconnect(wsConn)
	}

	slog.Debug("WebSocket client disconnected", "hub", h.name, "clients", clientCount)
}

// Broadcast sends data to all connected clients
func (h *WebSocketHub) Broadcast(data interface{}) {
	h.mu.RLock()
	clients := make([]*websocket.Conn, 0, len(h.clients))
	for client := range h.clients {
		clients = append(clients, client)
	}
	h.mu.RUnlock()

	for _, client := range clients {
		if err := client.SetWriteDeadline(time.Now().Add(h.writeTimeout)); err != nil {
			slog.Warn("Failed to set write deadline", "error", err)
			continue
		}
		if err := client.WriteJSON(data); err != nil {
			// Remove disconnected client
			h.mu.Lock()
			delete(h.clients, client)
			h.mu.Unlock()
			if err := client.Close(); err != nil {
				slog.Warn("Failed to close client connection", "error", err)
			}
		} else {
			if err := client.SetWriteDeadline(time.Time{}); err != nil {
				slog.Warn("Failed to clear write deadline", "error", err)
			}
		}
	}
}

// ClientCount returns the number of connected clients
func (h *WebSocketHub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}
