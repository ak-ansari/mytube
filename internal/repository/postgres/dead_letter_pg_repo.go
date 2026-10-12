package postgres

import (
	"context"

	"github.com/ak-ansari/mytube/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DeadLetterRepo struct{ pool *pgxpool.Pool }

func NewDeadLetterRepo(pool *pgxpool.Pool) *DeadLetterRepo {
	return &DeadLetterRepo{pool: pool}
}

func (r *DeadLetterRepo) Insert(ctx context.Context, j *models.DeadLetterJob) error {
	if j.ID == uuid.Nil {
		j.ID = uuid.New()
	}
	return r.pool.QueryRow(ctx, `
        INSERT INTO dead_letter_jobs (id, source, video_id, stage, attempts, retryable, error, payload)
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
        RETURNING created_at
    `, j.ID, j.Source, j.VideoID, j.Stage, j.Attempts, j.Retryable, j.Error, j.Payload).Scan(&j.CreatedAt)
}

func (r *DeadLetterRepo) ListByVideo(ctx context.Context, videoID uuid.UUID) ([]*models.DeadLetterJob, error) {
	rows, err := r.pool.Query(ctx, `
        SELECT id, source, video_id, stage, attempts, retryable, error, payload, created_at
        FROM dead_letter_jobs WHERE video_id=$1 ORDER BY created_at DESC
    `, videoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*models.DeadLetterJob{}
	for rows.Next() {
		var j models.DeadLetterJob
		if err := rows.Scan(&j.ID, &j.Source, &j.VideoID, &j.Stage, &j.Attempts, &j.Retryable, &j.Error, &j.Payload, &j.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, &j)
	}
	return out, rows.Err()
}
