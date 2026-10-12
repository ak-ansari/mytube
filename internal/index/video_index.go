package index

import (
	"context"

	"github.com/ak-ansari/mytube/internal/models"
)

// Suggestion is one completion option: the suggested text and the id of the video it came from.
type Suggestion struct {
	ID   string
	Text string
}

type VideoIndex interface {
	InsertVideo(ctx context.Context, v *models.Video) error
	UpdateVideo(ctx context.Context, v *models.Video) error
	SearchVideo(ctx context.Context, query string, fields []string, offset, size int) ([]string, error)
	SuggestVideos(ctx context.Context, query string) ([]Suggestion, error)
	DeleteVideo(ctx context.Context, id string) error
}
