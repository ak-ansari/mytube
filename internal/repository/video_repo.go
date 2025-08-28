package repository

import (
	"context"

	"github.com/ak-ansari/mytube/internal/models"
)

type VideoRepository interface {
	InsertBasic(ctx context.Context, v models.Video) error
	UpdateMeta(ctx context.Context, videoId string, sha string, dur int, vcodec, acodec string, w, h int) error
	UpdateStatus(ctx context.Context, videoId string, status models.VideoStatus) error
	UpdatePublish(ctx context.Context, videoId string, qualities []string, manifest string, thumbs any) error
	Get(ctx context.Context, videoId string) (*models.Video, error)
}
