package pubsub

type Hub struct {
	messages chan []byte
}

func NewHub(bufferSize int) *Hub {
	if bufferSize <= 0 {
		bufferSize = 1
	}

	return &Hub{
		messages: make(chan []byte, bufferSize),
	}
}

func (h *Hub) PublishLocal(message []byte) {
	if h == nil {
		return
	}

	select {
	case h.messages <- message:
	default:
	}
}

func (h *Hub) Messages() <-chan []byte {
	if h == nil {
		return nil
	}

	return h.messages
}
