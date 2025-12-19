package index

import (
	"context"

	"github.com/ak-ansari/mytube/internal/models"
)

type VideoIndex interface {
	InsertVideo(ctx context.Context, v *models.Video) error
	UpdateVideo(ctx context.Context, v *models.Video) error
	SearchVideo(ctx context.Context, query string, fields []string, offset, size int) ([]string, error)
	SuggestVideos(ctx context.Context, query string) ([]string, error)
}
