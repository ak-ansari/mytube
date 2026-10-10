package models

import (
	"time"

	"github.com/google/uuid"
)

// ChannelSummary is the uploader/author info embedded in video cards and comments.
type ChannelSummary struct {
	ID          uuid.UUID `json:"id"`
	UserName    string    `json:"user_name"`
	ChannelName *string   `json:"channel_name"`
	Avatar      *string   `json:"avatar"`     // object key
	AvatarURL   string    `json:"avatar_url"` // presigned url, filled by the service
}

// Channel is a user's public channel page.
type Channel struct {
	ChannelSummary
	Description  *string   `json:"description"`
	Subscribers  int64     `json:"subscribers"`
	VideoCount   int64     `json:"video_count"`
	IsSubscribed bool      `json:"is_subscribed"`
	CreatedAt    time.Time `json:"created_at"`
}

// Profile is the signed-in user's own account.
type Profile struct {
	Channel
	Email *string `json:"email"`
}

// VideoCard is a video as shown in lists (home, search, channel, studio).
type VideoCard struct {
	ID              uuid.UUID        `json:"id"`
	Title           *string          `json:"title"`
	Description     *string          `json:"description"`
	Thumbnail       *string          `json:"thumbnail"`     // object key (user upload, else generated)
	ThumbnailURL    string           `json:"thumbnail_url"` // presigned url, filled by the service
	Visibility      *VideoVisibility `json:"visibility"`
	Status          VideoStatus      `json:"status"`
	Views           int64            `json:"views"`
	DurationSeconds *int             `json:"duration_seconds"`
	CreatedAt       time.Time        `json:"created_at"`
	UpdatedAt       time.Time        `json:"updated_at"`
	Channel         ChannelSummary   `json:"channel"`
}

// VideoDetails is everything the watch page needs.
type VideoDetails struct {
	VideoCard
	ManifestPath       *string  `json:"manifest_path"`
	ManifestURL        string   `json:"manifest_url"`
	AvailableQualities []string `json:"available_qualities"`
	Likes              int64    `json:"likes"`
	Dislikes           int64    `json:"dislikes"`
	MyReaction         int      `json:"my_reaction"` // 1, -1 or 0
	CommentCount       int64    `json:"comment_count"`
	Subscribers        int64    `json:"subscribers"`
	IsSubscribed       bool     `json:"is_subscribed"`
}

type Comment struct {
	ID        uuid.UUID      `json:"id"`
	VideoID   uuid.UUID      `json:"video_id"`
	Content   string         `json:"content"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	Author    ChannelSummary `json:"author"`
}

// Page is a paginated list response.
type Page[T any] struct {
	Items   []T  `json:"items"`
	Page    int  `json:"page"`
	Size    int  `json:"size"`
	HasMore bool `json:"has_more"`
}

func NewPage[T any](items []T, page, size int) Page[T] {
	hasMore := len(items) > size
	if hasMore {
		items = items[:size]
	}
	if items == nil {
		items = []T{}
	}
	return Page[T]{Items: items, Page: page, Size: size, HasMore: hasMore}
}
