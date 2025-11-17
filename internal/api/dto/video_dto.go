package dto

import "github.com/ak-ansari/mytube/internal/models"

type UploadVideoDto struct {
	Size     int64  `json:"size"`
	Filename string `json:"file_name"`
}

type VideoConfirmDto struct {
	Thumbnail   string                 `json:"thumbnail"`             // address to video thumbnail file
	Title       string                 `json:"title"`                 // title of the video
	Description string                 `json:"description,omitempty"` // description about video
	Visibility  models.VideoVisibility `json:"visibility"`            // "public", "private", "unlisted"
}
