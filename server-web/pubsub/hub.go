package pubsub

import (
	"log"
)

type Hub struct {
	messages chan []byte
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
	select {
	case h.messages <- message:
	default:
		log.Printf("pubsub hub: message channel full, alert dropped")
	}
}
