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
        INSERT INTO videos (id, file_key, status, stage ,user_id)
        VALUES ($1,$2,$3,$4,$5)
    `, v.ID, v.FileKey, v.Status, v.Stage, v.UserID)
	return err
}
func (r *VideoRepo) UpdateVideo(ctx context.Context, v *models.Video) error {
	_, err := r.pool.Exec(ctx, `
        UPDATE videos SET title=$2, thumbnail=$3, description=$4, visibility=$5 WHERE id=$1
    `, v.ID, v.Title, v.Thumbnail, v.Description, v.Visibility)
	return err
}

func (r *VideoRepo) Get(ctx context.Context, id string) (*models.Video, error) {
	parsedId, _ := uuid.Parse(id)
	row := r.pool.QueryRow(ctx, `
        SELECT id, user_id, file_key, thumbnail, title, description, visibility, status, stage,created_at, updated_at
        FROM videos WHERE id=$1
    `, parsedId)
	var v models.Video
	if err := row.Scan(&v.ID, &v.UserID, &v.FileKey, &v.Thumbnail, &v.Title, &v.Description, &v.Visibility, &v.Status, &v.Stage, &v.CreatedAt, &v.UpdatedAt); err != nil {
		return nil, err
	}
	return &v, nil
}
func (r *VideoRepo) GetByIds(ctx context.Context, ids []string) ([]*models.Video, error) {
	parsedIds := make([]uuid.UUID, 0, len(ids))

	for _, id := range ids {
		parsedId, err := uuid.Parse(id)
		if err != nil {
			return nil, err
		}
		parsedIds = append(parsedIds, parsedId)
	}

	rows, err := r.pool.Query(ctx, `
        SELECT id, user_id, file_key, thumbnail, title, description, visibility, status, stage, created_at, updated_at
        FROM videos 
        WHERE id = ANY($1)
    `, parsedIds)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	videos := make([]*models.Video, 0)

	for rows.Next() {
		var v models.Video
		if err := rows.Scan(
			&v.ID, &v.UserID, &v.FileKey, &v.Thumbnail, &v.Title, &v.Description,
			&v.Visibility, &v.Status, &v.Stage, &v.CreatedAt, &v.UpdatedAt,
		); err != nil {
			return nil, err
		}
		videos = append(videos, &v)
	}

	if rows.Err() != nil {
		return nil, rows.Err()
	}

	return videos, nil
}

func (r *VideoRepo) UpdateState(ctx context.Context, videoId string, stage int, status models.VideoStatus) error {
	parsedId, _ := uuid.Parse(videoId)
	_, err := r.pool.Exec(ctx, `UPDATE videos SET stage=$2, status=$3 WHERE id=$1`, parsedId, stage, status)
	if err != nil {
		return err
	}
	return nil
}

func (r *VideoRepo) AdvanceState(ctx context.Context, videoId string, fromStage, toStage int, status models.VideoStatus) (bool, error) {
	parsedId, _ := uuid.Parse(videoId)
	// failed videos keep their stage number, so exclude them explicitly
	tag, err := r.pool.Exec(ctx, `
        UPDATE videos SET stage=$3, status=$4
        WHERE id=$1 AND stage=$2 AND status NOT IN ('phase_1_failed', 'phase_2_failed')
    `, parsedId, fromStage, toStage, status)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}
