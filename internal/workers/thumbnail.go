package workers

import (
	"context"
	"fmt"

	"github.com/ak-ansari/mytube/internal/jobs"
)

type Thumbnail struct {
}

func NewThumbnail() *Thumbnail {
	return &Thumbnail{}
}

func (c *Thumbnail) Handle(ctx context.Context, payload jobs.JobPayload) error {
	fmt.Printf("thumbnail processing started [videoId: %s] \n", payload.VideoID)
	fmt.Printf("thumbnail processing finished [videoId: %s] \n", payload.VideoID)
	return nil
}
