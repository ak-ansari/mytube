package fakes

import (
	"context"
	"sync"
	"time"

	"github.com/ak-ansari/mytube/internal/models"
	"github.com/ak-ansari/mytube/internal/repository"
	"github.com/google/uuid"
)

// DeadLetterRepo is an in-memory repository.DeadLetterRepository. Insert assigns an id (when nil)
// and created_at, like the Postgres impl; rows are stored as copies.
type DeadLetterRepo struct {
	Errs  Errs
	Calls Calls
	mu    sync.Mutex
	rows  []models.DeadLetterJob
}

var _ repository.DeadLetterRepository = (*DeadLetterRepo)(nil)

func NewDeadLetterRepo() *DeadLetterRepo { return &DeadLetterRepo{} }

func (r *DeadLetterRepo) Insert(_ context.Context, j *models.DeadLetterJob) error {
	r.Calls.add("Insert")
	if err := r.Errs.get("Insert"); err != nil {
		return err
	}
	if j.ID == uuid.Nil {
		j.ID = uuid.New()
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	// strictly increasing timestamps keep "newest first" deterministic
	j.CreatedAt = time.Now()
	if n := len(r.rows); n > 0 && !j.CreatedAt.After(r.rows[n-1].CreatedAt) {
		j.CreatedAt = r.rows[n-1].CreatedAt.Add(time.Microsecond)
	}
	r.rows = append(r.rows, *j)
	return nil
}

func (r *DeadLetterRepo) ListByVideo(_ context.Context, videoID uuid.UUID) ([]*models.DeadLetterJob, error) {
	r.Calls.add("ListByVideo")
	if err := r.Errs.get("ListByVideo"); err != nil {
		return nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []*models.DeadLetterJob{}
	for i := len(r.rows) - 1; i >= 0; i-- {
		if r.rows[i].VideoID != nil && *r.rows[i].VideoID == videoID {
			cp := r.rows[i]
			out = append(out, &cp)
		}
	}
	return out, nil
}

// All returns copies of every dead letter, oldest first.
func (r *DeadLetterRepo) All() []models.DeadLetterJob {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]models.DeadLetterJob(nil), r.rows...)
}
