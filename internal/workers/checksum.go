package workers

import (
	"context"

	"github.com/ak-ansari/mytube/internal/jobs"
	"github.com/ak-ansari/mytube/internal/pkg/logger"
)

type Checksum struct {
	log logger.Logger
}

func NewChecksum(log logger.Logger) *Checksum {
	return &Checksum{
		log: log,
	}
}

func (c *Checksum) Handle(ctx context.Context, payload jobs.JobPayload) error {
	c.log.Info("Checksum validation started",
		logger.String("videoId", payload.VideoID))

	// TODO: implement real checksum validation here

	c.log.Success("Checksum validation finished",
		logger.String("videoId", payload.VideoID))

	return nil
}
