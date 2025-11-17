package postgres

import (
	"context"

	"github.com/ak-ansari/mytube/internal/models"
	"github.com/google/uuid"
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
func (r *VideoRepo) UpdateVideo(ctx context.Context, v *models.Video) error {
	_, err := r.pool.Exec(ctx, `
        UPDATE videos SET title=$2, thumbnail=$3, description=$4, visibility=$5, status=$6 WHERE id=$1
    `, v.ID, v.Title, v.Thumbnail, v.Description, v.Visibility, v.Status)
	return err
}

func (r *VideoRepo) Get(ctx context.Context, id string) (*models.Video, error) {
	parsedId, _ := uuid.Parse(id)
	row := r.pool.QueryRow(ctx, `
        SELECT id, video_id, user_id, file_key, thumbnail, title, description, visibility, status, created_at, updated_at
        FROM videos WHERE id=$1
    `, parsedId)
	var v models.Video
	if err := row.Scan(&v.ID, &v.VideoId, &v.UserID, &v.FileKey, &v.Thumbnail, &v.Title, &v.Description, &v.Visibility, &v.Status, &v.CreatedAt, &v.UpdatedAt); err != nil {
		return nil, err
	}
	return &v, nil
}
