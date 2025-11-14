package repository

import (
	"context"

	"github.com/ak-ansari/mytube/internal/models"
)

type VideoMetadataRepository interface {
	InsertBasic(ctx context.Context, v *models.VideoMetadata) error
	UpdateMeta(ctx context.Context, videoId string, sha string, dur int, vcodec, acodec string, w, h int) error
	UpdateQualities(ctx context.Context, videoId string, qualities []string) error
	UpdateManifest(ctx context.Context, videoId string, manifest string) error
	UpdateThumbnail(ctx context.Context, videoId string, thumbnailKey string) error
	Get(ctx context.Context, videoId string) (*models.VideoMetadata, error)
}
