package workers

import (
	"context"
	"fmt"

	"github.com/ak-ansari/mytube/internal/jobs"
)

type Checksum struct {
}

func NewChecksum() *Checksum {
	return &Checksum{}
}

func (c *Checksum) Handle(ctx context.Context, payload jobs.JobPayload) error {
	fmt.Printf("checksum validation started [videoId: %s] \n", payload.VideoID)
	fmt.Printf("checksum validation finished [videoId: %s] \n", payload.VideoID)
	return nil
}
