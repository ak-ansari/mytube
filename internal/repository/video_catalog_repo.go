package repository

import (
	"context"

	"github.com/ak-ansari/mytube/internal/models"
	"github.com/google/uuid"
)

// VideoFilter narrows ListPublished. Zero values mean "no filter".
type VideoFilter struct {
	IDs          []uuid.UUID // keep only these ids, in this order (search results)
	ChannelID    *uuid.UUID
	SubscriberID *uuid.UUID // videos from channels this user subscribes to
	LikedBy      *uuid.UUID
	ExcludeID    *uuid.UUID
}

// VideoCatalogRepository serves the read/write needs of the public catalog and the creator studio.
type VideoCatalogRepository interface {
	ListPublished(ctx context.Context, f VideoFilter, offset, limit int) ([]*models.VideoCard, error)
	ListByOwner(ctx context.Context, ownerID uuid.UUID, offset, limit int) ([]*models.VideoCard, error)
	GetDetails(ctx context.Context, id uuid.UUID, viewerID *uuid.UUID) (*models.VideoDetails, error)
	GetOwnerID(ctx context.Context, id uuid.UUID) (uuid.UUID, error)
	UpdateDetails(ctx context.Context, v *models.Video) error
	Delete(ctx context.Context, id uuid.UUID) error
	IncrementViews(ctx context.Context, id uuid.UUID) error
	SetReaction(ctx context.Context, videoID, userID uuid.UUID, value int) error
}

type CommentRepository interface {
	List(ctx context.Context, videoID uuid.UUID, offset, limit int) ([]*models.Comment, error)
	Get(ctx context.Context, id uuid.UUID) (*models.Comment, error)
	Insert(ctx context.Context, id, videoID, userID uuid.UUID, content string) error
	Delete(ctx context.Context, id uuid.UUID) error
}
