package repository

import (
	"context"

	"github.com/ak-ansari/mytube/internal/models"
	"github.com/google/uuid"
)

type DeadLetterRepository interface {
	Insert(ctx context.Context, j *models.DeadLetterJob) error
	// ListByVideo returns a video's dead letters, newest first.
	ListByVideo(ctx context.Context, videoID uuid.UUID) ([]*models.DeadLetterJob, error)
}
