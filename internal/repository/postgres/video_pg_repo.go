package postgres

import (
	"context"
	"fmt"

	"github.com/ak-ansari/mytube/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type VideoRepo struct{ pool *pgxpool.Pool }

func NewVideoRepo(pool *pgxpool.Pool) *VideoRepo {
	return &VideoRepo{pool: pool}
}

func (r *VideoRepo) InsertBasic(ctx context.Context, v models.Video) error {
	_, err := r.pool.Exec(ctx, `
        INSERT INTO videos (id, filename, original_object_key, status)
        VALUES ($1,$2,$3,$4)
    `, v.ID, v.Filename, v.OriginalObjectKey, v.Status)
	return err
}

func (r *VideoRepo) UpdateMeta(ctx context.Context, videoId string, sha string, dur int, vcodec string, acodec string, w int, h int, status models.VideoStatus) error {
	id, _ := uuid.Parse(videoId)
	_, err := r.pool.Exec(ctx, `
        UPDATE videos SET sha256=$2, duration_seconds=$3, codec_video=$4, codec_audio=$5, width=$6, height=$7, status=$8,updated_at=now() WHERE id=$1
    `, id, sha, dur, vcodec, acodec, w, h, status)
	return err
}
func (r *VideoRepo) UpdateQualities(ctx context.Context, videoId string, qualities []string, status models.VideoStatus) error {
	id, _ := uuid.Parse(videoId)
	_, err := r.pool.Exec(ctx, `
        UPDATE videos SET available_qualities=$2, status=$3, updated_at=now() WHERE id=$1
    `, id, qualities, status)
	return err
}
func (r *VideoRepo) UpdateManifest(ctx context.Context, videoId string, manifest string) error {
	id, _ := uuid.Parse(videoId)
	_, err := r.pool.Exec(ctx, `
        UPDATE videos SET manifest_path=$2,updated_at=now() WHERE id=$1
    `, id, manifest)
	return err
}
func (r *VideoRepo) UpdateStatus(ctx context.Context, videoId string, status models.VideoStatus) error {
	id, err := uuid.Parse(videoId)
	if err != nil {
		return fmt.Errorf("invalid videoId: %w", err)
	}

	query := "UPDATE videos SET status=$2,updated_at=now() WHERE id=$1"

	_, err = r.pool.Exec(ctx, query, id, status)
	return err
}

func (r *VideoRepo) UpdateThumbnail(ctx context.Context, videoId string, thumbnailKey string) error {
	id, _ := uuid.Parse(videoId)
	_, err := r.pool.Exec(ctx, `
        UPDATE videos SET thumbnail=$2, updated_at=now() WHERE id=$1
    `, id, thumbnailKey)
	return err
}

func (r *VideoRepo) Get(ctx context.Context, videoId string) (*models.Video, error) {
	id, _ := uuid.Parse(videoId)
	row := r.pool.QueryRow(ctx, `
        SELECT id, filename, original_object_key, sha256, duration_seconds, codec_video, codec_audio, width, height, status, available_qualities, manifest_path, thumbnail, created_at, updated_at
        FROM videos WHERE id=$1
    `, id)
	var v models.Video
	if err := row.Scan(&v.ID, &v.Filename, &v.OriginalObjectKey, &v.SHA256, &v.DurationSeconds, &v.CodecVideo, &v.CodecAudio, &v.Width, &v.Height, &v.Status, &v.AvailableQualities, &v.ManifestPath, &v.Thumbnail, &v.CreatedAt, &v.UpdatedAt); err != nil {
		return nil, err
	}
	return &v, nil
}
