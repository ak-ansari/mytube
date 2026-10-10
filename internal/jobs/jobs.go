package jobs

import "github.com/ak-ansari/mytube/internal/models"

type JobPayload struct {
	VideoID string             `json:"video_id"`
	Stage   int                `json:"stage"`
	Status  models.VideoStatus `json:"status"`
	// Attempt counts earlier failed runs of this stage (0 on the first run); the runner bumps it on retry.
	Attempt int `json:"attempt,omitempty"`
}
