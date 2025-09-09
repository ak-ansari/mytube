package workers

import (
	"context"
	"fmt"

	"github.com/ak-ansari/mytube/internal/jobs"
	"github.com/ak-ansari/mytube/internal/models"
	"github.com/ak-ansari/mytube/internal/services"
)

type Publish struct {
	service *services.VideoService
}

func NewPublish(s *services.VideoService) *Publish {
	return &Publish{
		service: s,
	}
}

func (p *Publish) Handle(ctx context.Context, payload jobs.JobPayload) error {
	fmt.Printf("publish process started [videoId: %s] \n", payload.VideoID)
	if err := p.service.UpdateStatus(ctx, payload.VideoID, models.StatusReady); err != nil {
		return err
	}
	// TODO notify the user that video is processed
	fmt.Printf("🔥🔥 publish process finished [videoId: %s] 🔥🔥 \n", payload.VideoID)
	return nil
}
