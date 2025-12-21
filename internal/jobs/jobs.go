package jobs

import "github.com/ak-ansari/mytube/models"

type JobPayload struct {
	VideoID string             `json:"video_id"`
	Stage   int                `json:"stage"`
	Status  models.VideoStatus `json:"status"`
}
