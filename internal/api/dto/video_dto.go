package dto

import "github.com/ak-ansari/mytube/models"

type UploadVideoDto struct {
	Size     int64  `json:"size" validate:"required,gt=0"`
	Filename string `json:"file_name" validate:"required,min=3"`
}

type VideoConfirmDto struct {
	Thumbnail   string                 `json:"thumbnail" validate:"required,min=3"`                          // address to video thumbnail file
	Title       string                 `json:"title" validate:"required,min=3"`                              // title of the video
	Description string                 `json:"description,omitempty"`                                        // description about video
	Visibility  models.VideoVisibility `json:"visibility" validate:"required,oneof=public private unlisted"` // "public", "private", "unlisted"
}
