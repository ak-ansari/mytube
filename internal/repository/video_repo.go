package repository

import (
	"context"

	"github.com/ak-ansari/mytube/internal/models"
	"github.com/google/uuid"
)

type VideoRepository interface {
	InsertBasic(ctx context.Context, v models.Video) error
	UpdateMeta(ctx context.Context, id uuid.UUID, sha string, dur int, vcodec, acodec string, w, h int) error
	UpdateStatus(ctx context.Context, id uuid.UUID, status models.VideoStatus) error
	UpdatePublish(ctx context.Context, id uuid.UUID, qualities []string, manifest string, thumbs any) error
	Get(ctx context.Context, id uuid.UUID) (*models.Video, error)
}
