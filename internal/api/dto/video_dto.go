package dto

import "github.com/ak-ansari/mytube/internal/models"

// gin validates `binding` tags (go-playground/validator) on ShouldBind*.
type UploadVideoDto struct {
	Size     int64  `json:"size" binding:"required,gt=0,lte=10737418240"` // 10 GiB, matches nginx client_max_body_size 10G
	Filename string `json:"file_name" binding:"required,min=3"`
}

type VideoConfirmDto struct {
	Thumbnail   string                 `json:"thumbnail"`                                                   // custom thumbnail key under thumbnails/{videoId}/; "" keeps the auto-generated one
	Title       string                 `json:"title" binding:"required,min=3"`                              // title of the video (trimmed 3-100 chars, checked in the service)
	Description string                 `json:"description,omitempty" binding:"max=5000"`                    // description about video
	Visibility  models.VideoVisibility `json:"visibility" binding:"required,oneof=public private unlisted"` // "public", "private", "unlisted"
}
