package repository

import (
	"context"

	"github.com/ak-ansari/mytube/internal/models"
)

type VideoRepository interface {
	InsertBasic(ctx context.Context, v models.Video) error
	UpdateMeta(ctx context.Context, videoId string, sha string, dur int, vcodec, acodec string, w, h int, status models.VideoStatus) error
	UpdateStatus(ctx context.Context, videoId string, status models.VideoStatus) error
	UpdateQualities(ctx context.Context, videoId string, qualities []string, status models.VideoStatus) error
	UpdateManifest(ctx context.Context, videoId string, manifest string) error
	UpdatePublish(ctx context.Context, videoId string, qualities []string, manifest string, thumbs any, status models.VideoStatus) error
	Get(ctx context.Context, videoId string) (*models.Video, error)
}
