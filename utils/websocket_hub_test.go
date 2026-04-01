package utils

import (
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestWebSocketHubBroadcastSerializesConcurrentWriters(t *testing.T) {
	t.Parallel()

	runtime.GOMAXPROCS(8)

	hub := NewWebSocketHub("test")
	server := httptest.NewServer(http.HandlerFunc(hub.HandleConnection))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Dial() error = %v", err)
	}
	defer conn.Close() //nolint:errcheck

	done := make(chan struct{})
	defer close(done)

	go func() {
		for {
			select {
			case <-done:
				return
			default:
				if _, _, err := conn.ReadMessage(); err != nil {
					return
				}
			}
		}
	}()

	time.Sleep(100 * time.Millisecond)

	for round := 0; round < 100; round++ {
		start := make(chan struct{})
		var wg sync.WaitGroup

		for writer := 0; writer < 8; writer++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				<-start
				hub.Broadcast(map[string]any{
					"round":  round,
					"writer": id,
					"text":   "test-message",
				})
			}(writer)
		}

		close(start)
		wg.Wait()
	}
}
