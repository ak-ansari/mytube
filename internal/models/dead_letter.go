package models

import (
	"time"

	"github.com/google/uuid"
)

type DeadLetterSource string

const (
	DeadLetterSourceJob         DeadLetterSource = "job"          // a pipeline stage job
	DeadLetterSourceBucketEvent DeadLetterSource = "bucket_event" // a MinIO upload event
)

// DeadLetterJob is pipeline work that failed for good (row of dead_letter_jobs).
type DeadLetterJob struct {
	ID        uuid.UUID        `json:"id"`
	Source    DeadLetterSource `json:"source"`
	VideoID   *uuid.UUID       `json:"video_id,omitempty"`
	Stage     *int             `json:"stage,omitempty"`
	Attempts  int              `json:"attempts"`
	Retryable bool             `json:"retryable"` // true: retries ran out; false: permanent error
	Error     string           `json:"error"`
	Payload   string           `json:"payload"`
	CreatedAt time.Time        `json:"created_at"`
}
