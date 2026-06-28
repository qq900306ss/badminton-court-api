// Package realtime is an in-process WebSocket hub: one "room" per session.
// The always-on Fly server holds the connections in memory and broadcasts a
// tiny "changed" nudge whenever a session mutates, so clients refetch instantly
// instead of waiting for the poll. (This is why the backend moved off Lambda —
// Lambda can't hold long-lived connections; one always-on machine can.)
//
// Requires the app to run as a SINGLE machine (in-memory rooms aren't shared
// across machines) — see fly.toml min/max = 1.
package realtime

import (
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	// the WS only carries non-sensitive "changed" nudges; real data still goes
	// through the authenticated REST API. Origin is allowed at the app layer.
	CheckOrigin: func(r *http.Request) bool { return true },
}

type client struct {
	send chan []byte
}

// Hub holds WS connections grouped by room (session id).
type Hub struct {
	mu    sync.RWMutex
	rooms map[string]map[*client]struct{}
}

// Default is the process-wide hub.
var Default = &Hub{rooms: map[string]map[*client]struct{}{}}

func (h *Hub) add(room string, c *client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.rooms[room] == nil {
		h.rooms[room] = map[*client]struct{}{}
	}
	h.rooms[room][c] = struct{}{}
}

func (h *Hub) remove(room string, c *client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if m := h.rooms[room]; m != nil {
		delete(m, c)
		if len(m) == 0 {
			delete(h.rooms, room)
		}
	}
}

// Broadcast sends msg to every client in a room (non-blocking; a slow client is
// skipped rather than stalling the others).
func (h *Hub) Broadcast(room string, msg []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.rooms[room] {
		select {
		case c.send <- msg:
		default:
		}
	}
}

// Serve upgrades an HTTP request to a WebSocket joined to `room` and blocks
// until the connection closes.
func (h *Hub) Serve(w http.ResponseWriter, r *http.Request, room string) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	c := &client{send: make(chan []byte, 16)}
	h.add(room, c)
	done := make(chan struct{})

	// writer: pump queued messages + periodic ping to keep the connection alive
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case msg, ok := <-c.send:
				if !ok {
					return
				}
				_ = ws.SetWriteDeadline(time.Now().Add(10 * time.Second))
				if err := ws.WriteMessage(websocket.TextMessage, msg); err != nil {
					return
				}
			case <-ticker.C:
				_ = ws.SetWriteDeadline(time.Now().Add(10 * time.Second))
				if err := ws.WriteMessage(websocket.PingMessage, nil); err != nil {
					return
				}
			case <-done:
				return
			}
		}
	}()

	// reader: we don't expect client messages; this just detects disconnect
	ws.SetReadLimit(512)
	for {
		if _, _, err := ws.ReadMessage(); err != nil {
			break
		}
	}

	h.remove(room, c) // serialized with Broadcast via the mutex → no send-after on a gone client
	close(done)
	_ = ws.Close()
}
