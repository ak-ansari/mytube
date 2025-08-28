package postgres

import (
	"context"
	"encoding/json"

	"github.com/ak-ansari/mytube/internal/models"
	"github.com/ak-ansari/mytube/internal/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type VideoRepo struct{ pool *pgxpool.Pool }

func NewVideoRepo(pool *pgxpool.Pool) *VideoRepo { return &VideoRepo{pool: pool} }

var _ repository.VideoRepository = (*VideoRepo)(nil)

func (r *VideoRepo) InsertBasic(ctx context.Context, v models.Video) error {
	_, err := r.pool.Exec(ctx, `
        INSERT INTO videos (id, filename, original_object_key, status)
        VALUES ($1,$2,$3,$4)
    `, v.ID, v.Filename, v.OriginalObjectKey, v.Status)
	return err
}

func (r *VideoRepo) UpdateMeta(ctx context.Context, videoId string, sha string, dur int, vcodec string, acodec string, w int, h int) error {
	id, _ := uuid.Parse(videoId)
	_, err := r.pool.Exec(ctx, `
        UPDATE videos SET sha256=$2, duration_seconds=$3, codec_video=$4, codec_audio=$5, width=$6, height=$7, updated_at=now() WHERE id=$1
    `, id, sha, dur, vcodec, acodec, w, h)
	return err
}

func (r *VideoRepo) UpdateStatus(ctx context.Context, videoId string, status models.VideoStatus) error {
	id, _ := uuid.Parse(videoId)
	_, err := r.pool.Exec(ctx, `UPDATE videos SET status=$2, updated_at=now() WHERE id=$1`, id, status)
	return err
}

func (r *VideoRepo) UpdatePublish(ctx context.Context, videoId string, qualities []string, manifest string, thumbs any) error {
	id, _ := uuid.Parse(videoId)
	b, _ := json.Marshal(thumbs)
	_, err := r.pool.Exec(ctx, `
        UPDATE videos SET status='ready', available_qualities=$2, manifest_path=$3, thumbnails=$4, updated_at=now() WHERE id=$1
    `, id, qualities, manifest, b)
	return err
}

func (r *VideoRepo) Get(ctx context.Context, videoId string) (*models.Video, error) {
	id, _ := uuid.Parse(videoId)
	row := r.pool.QueryRow(ctx, `
        SELECT id, filename, original_object_key, sha256, duration_seconds, codec_video, codec_audio, width, height, status, available_qualities, manifest_path, thumbnails, created_at, updated_at
        FROM videos WHERE id=$1
    `, id)
	var v models.Video
	var thumbs []byte
	if err := row.Scan(&v.ID, &v.Filename, &v.OriginalObjectKey, &v.SHA256, &v.DurationSeconds, &v.CodecVideo, &v.CodecAudio, &v.Width, &v.Height, &v.Status, &v.AvailableQualities, &v.ManifestPath, &thumbs, &v.CreatedAt, &v.UpdatedAt); err != nil {
		return nil, err
	}
	v.ThumbnailsJSON = thumbs
	return &v, nil
}
