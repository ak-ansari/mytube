package workers

import (
	"context"
	"fmt"

	"github.com/ak-ansari/mytube/internal/jobs"
	"github.com/ak-ansari/mytube/internal/queue"
	// "github.com/google/uuid"
)

type Transcode struct {
	queue queue.Queue
}

func NewTranscoder(queue queue.Queue) *Transcode {
	return &Transcode{
		queue: queue,
	}
}

func (c *Transcode) Handle(ctx context.Context, payload jobs.JobPayload) error {
	fmt.Printf("transcoding started [videoId: %s] \n", payload.VideoID)
	// id, _ := uuid.Parse(payload.VideoID)
	fmt.Printf("transcoding finished [videoId: %s] \n", payload.VideoID)
	return nil
}
