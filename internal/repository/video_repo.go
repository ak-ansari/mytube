package repository

import (
	"context"

	"github.com/ak-ansari/mytube/internal/models"
)

type VideoRepository interface {
	InsertBasic(ctx context.Context, vm *models.Video) error
	UpdateVideo(ctx context.Context, vm *models.Video) error
	Get(ctx context.Context, id string) (*models.Video, error)
	GetByIds(ctx context.Context, id []string) ([]*models.Video, error)
	UpdateState(ctx context.Context, videoId string, stage int, status models.VideoStatus) error
	// AdvanceState moves the video from fromStage to toStage only if it is still at fromStage (and not failed).
	// It reports whether a row was updated, so concurrent callers advance a stage at most once.
	AdvanceState(ctx context.Context, videoId string, fromStage, toStage int, status models.VideoStatus) (bool, error)
}
