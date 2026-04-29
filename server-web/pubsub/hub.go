package pubsub

import (
	"log/slog"
	"sync"
)

type Hub struct {
	messages chan []byte
	mu       sync.RWMutex
	closed   bool
}

func NewHub(bufferSize int) *Hub {
	return &Hub{
		messages: make(chan []byte, bufferSize),
	}
}

func (h *Hub) Messages() <-chan []byte {
	return h.messages
}

func (h *Hub) PublishLocal(message []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.closed {
		return
	}
	select {
	case h.messages <- message:
	default:
		slog.Warn("pubsub hub: message channel full, alert dropped")
	}
}

func (h *Hub) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return
	}
	h.closed = true
	close(h.messages)
}
