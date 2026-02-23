package handler

import (
	"encoding/json"
	"log"
	"sync"

	"yt-dlp-web/internal/download"

	"github.com/gofiber/contrib/websocket"
)

// Hub manages WebSocket connections and broadcasts task updates.
type Hub struct {
	clients map[*websocket.Conn]bool
	mu      sync.Mutex
}

// NewHub creates and returns a new Hub.
func NewHub() *Hub {
	return &Hub{
		clients: make(map[*websocket.Conn]bool),
	}
}

// Register adds a client and sends the current task list.
func (h *Hub) Register(c *websocket.Conn, mgr *download.Manager) {
	tasks := mgr.List()
	data, _ := json.Marshal(map[string]interface{}{
		"type":  "init",
		"tasks": tasks,
	})
	h.mu.Lock()
	h.clients[c] = true
	_ = c.WriteMessage(websocket.TextMessage, data)
	h.mu.Unlock()
}

// Unregister removes a client.
func (h *Hub) Unregister(c *websocket.Conn) {
	h.mu.Lock()
	delete(h.clients, c)
	h.mu.Unlock()
}

// BroadcastTask sends a task update to all connected clients.
// Uses a full Mutex (not RLock) to safely handle client removal on error.
func (h *Hub) BroadcastTask(t *download.Task) {
	data, err := json.Marshal(map[string]interface{}{
		"type":  "update",
		"task": t.Snapshot(),
	})
	if err != nil {
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	for c := range h.clients {
		if err := c.WriteMessage(websocket.TextMessage, data); err != nil {
			log.Printf("[ws] write error, removing client: %v", err)
			c.Close()
			delete(h.clients, c)
		}
	}
}
