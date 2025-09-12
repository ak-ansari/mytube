package workers

import (
	"context"

	"github.com/ak-ansari/mytube/internal/jobs"
	"github.com/ak-ansari/mytube/internal/models"
	"github.com/ak-ansari/mytube/internal/pkg/logger"
	"github.com/ak-ansari/mytube/internal/services"
)

type Publish struct {
	service *services.VideoService
	log     logger.Logger
}

func NewPublish(s *services.VideoService, log logger.Logger) *Publish {
	return &Publish{
		service: s,
		log:     log,
	}
}

func (p *Publish) Handle(ctx context.Context, payload jobs.JobPayload) error {
	p.log.Info("Publish process started",
		logger.String("videoId", payload.VideoID))

	if err := p.service.UpdateStatus(ctx, payload.VideoID, models.StatusReady); err != nil {
		p.log.Error("Failed to update video status",
			logger.String("videoId", payload.VideoID),
			logger.Error(err))
		return err
	}

	// TODO: Notify the user that video is processed
	p.log.Success("🔥 Publish process finished",
		logger.String("videoId", payload.VideoID))

	return nil
}
