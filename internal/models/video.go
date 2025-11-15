package models

import (
	"time"

	"github.com/google/uuid"
)

type VideoStatus string

const (
	StatusUploading  VideoStatus = "uploading"
	StatusUploaded   VideoStatus = "uploaded"
	StatusValid      VideoStatus = "valid"
	StatusProcessing VideoStatus = "processing"
	StatusReady      VideoStatus = "ready"
	StatusFailed     VideoStatus = "failed"
)

type VideoVisibility string

const (
	VisibilityPublic   VideoVisibility = "public"
	VisibilityPrivate  VideoVisibility = "private"
	VisibilityUnlisted VideoVisibility = "unlisted"
)

type Video struct {
	ID          uuid.UUID        `json:"id"`                    // primary key
	VideoId     uuid.UUID        `json:"video_id"`              // video id ref to video_metadata table
	UserID      uuid.UUID        `json:"user_id"`               // uploader
	FileKey     string           `json:"file_key"`              // file address at cloude store
	Thumbnail   *string          `json:"thumbnail"`             // address to video thumbnail file
	Title       *string          `json:"title"`                 // title of the video
	Description *string          `json:"description,omitempty"` // description about video
	Visibility  *VideoVisibility `json:"visibility"`            // "public", "private", "unlisted"
	Status      VideoStatus      `json:"status"`                // "pending", "uploaded", "processing", "ready"
	CreatedAt   time.Time        `json:"created_at"`            // timestamp
	UpdatedAt   time.Time        `json:"updated_at"`            // timestamp
}
