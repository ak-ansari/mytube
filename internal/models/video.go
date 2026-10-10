package models

import (
	"time"

	"github.com/google/uuid"
)

type VideoStatus string

const (
	StatusUploadPending      VideoStatus = "upload_pending"
	StatusUploaded           VideoStatus = "uploaded"
	StatusValid              VideoStatus = "valid"
	StatusThumbnailGenerated VideoStatus = "thumbnail_generated"
	StatusTranscoded         VideoStatus = "transcoded"
	StatusSegmentGenerated   VideoStatus = "segment_generated"
	StatusChecksum           VideoStatus = "checksum"
	StatusPublished          VideoStatus = "published"
	StatusDone               VideoStatus = "done"
	StatusPhase1Failed       VideoStatus = "phase_1_failed"
	StatusPhase2Failed       VideoStatus = "phase_2_failed"
)

type VideoVisibility string

const (
	VisibilityPublic   VideoVisibility = "public"
	VisibilityPrivate  VideoVisibility = "private"
	VisibilityUnlisted VideoVisibility = "unlisted"
)

type Video struct {
	ID          uuid.UUID        `json:"id"`                    // primary key
	UserID      uuid.UUID        `json:"user_id"`               // uploader
	FileKey     string           `json:"file_key"`              // file address at cloude store
	Thumbnail   *string          `json:"thumbnail"`             // address to video thumbnail file
	Title       *string          `json:"title"`                 // title of the video
	Description *string          `json:"description,omitempty"` // description about video
	Visibility  *VideoVisibility `json:"visibility"`            // "public", "private", "unlisted"
	Status      VideoStatus      `json:"status"`                // "pending", "uploaded", "processing", "ready"
	Stage       int              `json:"stage"`                 // current pipeline stage of video
	CreatedAt   time.Time        `json:"created_at"`            // timestamp
	UpdatedAt   time.Time        `json:"updated_at"`            // timestamp
}
