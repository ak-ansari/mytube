package repository

import (
	"context"

	"github.com/ak-ansari/mytube/internal/models"
)

type VideoRepository interface {
	InsertBasic(ctx context.Context, vm *models.Video) error
	UpdateVideo(ctx context.Context, vm *models.Video) error
	Get(ctx context.Context, id string) (*models.Video, error)
	UpdateState(ctx context.Context, videoId string, stage int, status models.VideoStatus) error
}
