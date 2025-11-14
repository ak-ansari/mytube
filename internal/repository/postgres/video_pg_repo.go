package postgres

import (
	"context"

	"github.com/ak-ansari/mytube/internal/models"
	"github.com/jackc/pgx/v5/pgxpool"
)

type VideoRepo struct{ pool *pgxpool.Pool }

func NewVideoRepo(pool *pgxpool.Pool) *VideoRepo {
	return &VideoRepo{pool: pool}
}

func (r *VideoRepo) InsertBasic(ctx context.Context, v *models.Video) error {
	_, err := r.pool.Exec(ctx, `
        INSERT INTO videos (id, video_id, file_key, status ,user_id)
        VALUES ($1,$2,$3,$4,$5)
    `, v.ID, v.VideoId, v.FileKey, v.Status, v.UserID)
	return err
}
