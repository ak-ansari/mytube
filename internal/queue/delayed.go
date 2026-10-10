package queue

import (
	"context"
	"time"
)

// DelayedQueue can hold a message back until a delay has passed (used for retries with backoff).
type DelayedQueue interface {
	Queue
	// EnqueueAfter makes payload available on qname once delay has elapsed.
	EnqueueAfter(ctx context.Context, qname string, payload []byte, delay time.Duration) error
	// PromoteDue moves up to limit messages whose delay elapsed by now onto qname and returns how many moved.
	PromoteDue(ctx context.Context, qname string, now time.Time, limit int) (int, error)
}
