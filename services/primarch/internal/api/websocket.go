package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"

	"github.com/guaranaja/astartes-primaris/services/primarch/internal/domain"
)

// WSHub manages SSE (Server-Sent Events) connections and broadcasts events.
// SSE is used instead of WebSocket to avoid external dependencies — it provides
// the same real-time push capability and works natively in all browsers.
type WSHub struct {
	mu      sync.RWMutex
	clients map[chan string]bool
	logger  *slog.Logger
}

// NewWSHub creates a new event hub.
func NewWSHub() *WSHub {
	return &WSHub{
		clients: make(map[chan string]bool),
		logger:  slog.Default(),
	}
}

// HandleWebSocket serves an SSE stream (despite the name, kept for API compat).
// Connect via: const es = new EventSource("/ws")
func (h *WSHub) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	ch := make(chan string, 64)
	h.register(ch)
	defer h.unregister(ch)

	// Send initial connected event
	fmt.Fprintf(w, "event: connected\ndata: {\"status\":\"connected\"}\n\n")
	flusher.Flush()

	for {
		select {
		case <-r.Context().Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			fmt.Fprintf(w, "data: %s\n\n", msg)
			flusher.Flush()
		}
	}
}

// Broadcast sends an event to all connected SSE clients.
func (h *WSHub) Broadcast(event domain.SystemEvent) {
	data, err := json.Marshal(event)
	if err != nil {
		return
	}
	msg := string(data)

	h.mu.RLock()
	defer h.mu.RUnlock()

	for ch := range h.clients {
		select {
		case ch <- msg:
		default:
			// Client too slow, skip
			h.logger.Debug("sse client too slow, dropping event")
		}
	}
}

// ClientCount returns the number of connected SSE clients.
func (h *WSHub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

func (h *WSHub) register(ch chan string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clients[ch] = true
	h.logger.Info("sse client connected", "clients", len(h.clients))
}

func (h *WSHub) unregister(ch chan string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.clients, ch)
	close(ch)
	h.logger.Info("sse client disconnected", "clients", len(h.clients))
}
