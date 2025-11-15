package models

import (
	"time"

	"github.com/google/uuid"
)

type VideoMetadata struct {
	ID                 uuid.UUID `json:"id"` // primary key
	Filename           string    `json:"filename"`
	OriginalObjectKey  string    `json:"original_object_key"`
	SHA256             *string   `json:"sha256,omitempty"`
	DurationSeconds    *int      `json:"duration_seconds,omitempty"`
	Size               int64     `json:"size"`
	CodecVideo         *string   `json:"codec_video,omitempty"`
	CodecAudio         *string   `json:"codec_audio,omitempty"`
	Width              *int      `json:"width,omitempty"`
	Height             *int      `json:"height,omitempty"`
	ManifestPath       *string   `json:"manifest_path,omitempty"`
	AvailableQualities []string  `json:"available_qualities,omitempty"` // e.g., ["360p","720p"]
	Thumbnail          *string   `json:"thumbnail,omitempty"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}
