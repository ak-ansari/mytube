package queue

import "context"

type Message struct {
	Body []byte
}

type Queue interface {
	Enqueue(ctx context.Context, qname string, payload []byte) error
	Dequeue(ctx context.Context, qname string) ([]byte, error) // blocking-ish
	// DLQ handling is implementation-specific; keep interface minimal for swapability
}
