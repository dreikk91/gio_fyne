package core

type DeliveryResult struct {
	Status bool
}

type SharedMessage struct {
	Payload []byte
	ReplyCh chan DeliveryResult
}

type MessageQueue struct {
	ch chan SharedMessage
}

func NewMessageQueue(size int) *MessageQueue {
	if size < 1 {
		size = 1
	}
	return &MessageQueue{ch: make(chan SharedMessage, size)}
}

func (q *MessageQueue) Reader() <-chan SharedMessage {
	return q.ch
}

func (q *MessageQueue) Enqueue(msg SharedMessage) bool {
	select {
	case q.ch <- msg:
		return true
	default:
		return false
	}
}

func (q *MessageQueue) Close() {
	close(q.ch)
}
