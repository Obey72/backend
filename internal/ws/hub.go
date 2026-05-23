package ws

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
)

// hub broadcasts events to all connected sse clients
// the renderer connects once on startup and receives push updates
// instead of polling /status every second
type Hub struct {
	mu      sync.Mutex
	clients map[chan string]struct{}
}

func NewHub() *Hub {
	return &Hub{
		clients: make(map[chan string]struct{}),
	}
}

func (h *Hub) add(c chan string) {
	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.mu.Unlock()
}

func (h *Hub) remove(c chan string) {
	h.mu.Lock()
	delete(h.clients, c)
	h.mu.Unlock()
	close(c)
}

// send broadcasts a typed event to all connected clients
// non-blocking: slow/disconnected clients are dropped
func (h *Hub) Send(kind string, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	msg := fmt.Sprintf("event: %s\ndata: %s\n\n", kind, data)
	h.mu.Lock()
	var dead []chan string
	for c := range h.clients {
		select {
		case c <- msg:
		default:
			dead = append(dead, c)
		}
	}
	h.mu.Unlock()
	for _, c := range dead {
		h.remove(c)
	}
}

// conncount returns how many sse clients are currently connected
func (h *Hub) ConnCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.clients)
}

// handler is the http handler for get /events
// browsers connect here and receive a push stream
func (h *Hub) Handler(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	ch := make(chan string, 32)
	h.add(ch)
	defer h.remove(ch)

	ctx := r.Context()
	for {
		select {
		case msg, ok := <-ch:
			if !ok {
				return
			}
			fmt.Fprint(w, msg)
			flusher.Flush()
		case <-ctx.Done():
			return
		}
	}
}
