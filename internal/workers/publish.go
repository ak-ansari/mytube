package workers

import (
	"context"
	"fmt"

	"github.com/ak-ansari/mytube/internal/jobs"
)

type Publish struct {
}

func NewPublish() *Publish {
	return &Publish{}
}

func (c *Publish) Handle(ctx context.Context, payload jobs.JobPayload) error {
	fmt.Printf("publish process started [videoId: %s] \n", payload.VideoID)
	fmt.Printf("publish process finished [videoId: %s] \n", payload.VideoID)
	return nil
}
