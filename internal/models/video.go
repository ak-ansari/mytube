package models

import (
	"time"

	"github.com/google/uuid"
)

type VideoStatus string

const (
	StatusUploaded   VideoStatus = "uploaded"
	StatusValid      VideoStatus = "valid"
	StatusProcessing VideoStatus = "processing"
	StatusReady      VideoStatus = "ready"
	StatusFailed     VideoStatus = "failed"
)

type Video struct {
	ID                 uuid.UUID   `json:"id"`
	Filename           string      `json:"filename"`
	OriginalObjectKey  string      `json:"original_object_key"`
	SHA256             *string     `json:"sha256,omitempty"`
	DurationSeconds    *int        `json:"duration_seconds,omitempty"`
	CodecVideo         *string     `json:"codec_video,omitempty"`
	CodecAudio         *string     `json:"codec_audio,omitempty"`
	Width              *int        `json:"width,omitempty"`
	Height             *int        `json:"height,omitempty"`
	Status             VideoStatus `json:"status"`
	AvailableQualities []string    `json:"available_qualities,omitempty"`
	ManifestPath       *string     `json:"manifest_path,omitempty"`
	Thumbnail          *string     `json:"thumbnail,omitempty"`
	CreatedAt          time.Time   `json:"created_at"`
	UpdatedAt          time.Time   `json:"updated_at"`
}
