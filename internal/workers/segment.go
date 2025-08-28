package workers

import (
	"context"
	"fmt"

	"github.com/ak-ansari/mytube/internal/jobs"
)

type Segment struct {
}

func NewSegment() *Segment {
	return &Segment{}
}

func (c *Segment) Handle(ctx context.Context, payload jobs.JobPayload) error {
	fmt.Printf("segment processing started [videoId: %s] \n", payload.VideoID)
	fmt.Printf("segment processing finished [videoId: %s] \n", payload.VideoID)
	return nil
}
