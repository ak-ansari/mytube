package postgres

import (
	"context"

	"github.com/ak-ansari/mytube/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type VideoMetadataRepo struct{ pool *pgxpool.Pool }

func NewVideoMetadataRepo(pool *pgxpool.Pool) *VideoMetadataRepo {
	return &VideoMetadataRepo{pool: pool}
}

func (r *VideoMetadataRepo) InsertBasic(ctx context.Context, v *models.VideoMetadata) error {
	_, err := r.pool.Exec(ctx, `
        INSERT INTO video_metadata (id, filename, original_object_key,size,video_id)
        VALUES ($1,$2,$3,$4,$5)
    `, v.ID, v.Filename, v.OriginalObjectKey, v.Size, v.VideoId)
	return err
}

func (r *VideoMetadataRepo) UpdateMeta(ctx context.Context, videoId string, sha string, dur int, vcodec string, acodec string, w int, h int) error {
	id, _ := uuid.Parse(videoId)
	_, err := r.pool.Exec(ctx, `
        UPDATE video_metadata SET sha256=$2, duration_seconds=$3, codec_video=$4, codec_audio=$5, width=$6, height=$7,updated_at=now() WHERE id=$1
    `, id, sha, dur, vcodec, acodec, w, h)
	return err
}
func (r *VideoMetadataRepo) UpdateQualities(ctx context.Context, videoId string, qualities []string) error {
	id, _ := uuid.Parse(videoId)
	_, err := r.pool.Exec(ctx, `
        UPDATE video_metadata SET available_qualities=$2, updated_at=now() WHERE id=$1
    `, id, qualities)
	return err
}
func (r *VideoMetadataRepo) UpdateManifest(ctx context.Context, videoId string, manifest string) error {
	id, _ := uuid.Parse(videoId)
	_, err := r.pool.Exec(ctx, `
        UPDATE video_metadata SET manifest_path=$2,updated_at=now() WHERE id=$1
    `, id, manifest)
	return err
}

func (r *VideoMetadataRepo) UpdateThumbnail(ctx context.Context, videoId string, thumbnailKey string) error {
	id, _ := uuid.Parse(videoId)
	_, err := r.pool.Exec(ctx, `
        UPDATE video_metadata SET thumbnail=$2, updated_at=now() WHERE id=$1
    `, id, thumbnailKey)
	return err
}

func (r *VideoMetadataRepo) Get(ctx context.Context, videoId string) (*models.VideoMetadata, error) {
	id, _ := uuid.Parse(videoId)
	row := r.pool.QueryRow(ctx, `
        SELECT id, filename, original_object_key, duration_seconds, codec_video, codec_audio, width, height, available_qualities, manifest_path, thumbnail, created_at, updated_at
        FROM video_metadata WHERE id=$1 OR video_id=$1
    `, id)
	var v models.VideoMetadata
	if err := row.Scan(&v.ID, &v.Filename, &v.OriginalObjectKey, &v.DurationSeconds, &v.CodecVideo, &v.CodecAudio, &v.Width, &v.Height, &v.AvailableQualities, &v.ManifestPath, &v.Thumbnail, &v.CreatedAt, &v.UpdatedAt); err != nil {
		return nil, err
	}
	return &v, nil
}
func (r *VideoMetadataRepo) GetByKey(ctx context.Context, key string) (string, error) {
	row := r.pool.QueryRow(ctx, `
        SELECT id
        FROM video_metadata WHERE original_object_key=$1
    `, key)
	var v models.VideoMetadata
	if err := row.Scan(&v.ID); err != nil {
		return "", err
	}
	return v.ID.String(), nil
}
