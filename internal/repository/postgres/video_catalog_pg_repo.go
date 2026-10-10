package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/ak-ansari/mytube/internal/models"
	"github.com/ak-ansari/mytube/internal/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// a video is visible to everyone once processing finished and it is public
const publishedVideoCond = `v.status = 'published' AND v.visibility = 'public'`

const videoCardSelect = `
	SELECT v.id, v.title, v.description, COALESCE(NULLIF(v.thumbnail, ''), vm.thumbnail), v.visibility, v.status,
		v.views, vm.duration_seconds, v.created_at, v.updated_at,
		u.id, u.user_name, u.channel_name, u.avatar`

const videoCardFrom = `
	FROM videos v
	JOIN users u ON u.id = v.user_id
	LEFT JOIN video_metadata vm ON vm.video_id = v.id`

type VideoCatalogRepo struct{ pool *pgxpool.Pool }

func NewVideoCatalogRepo(pool *pgxpool.Pool) *VideoCatalogRepo {
	return &VideoCatalogRepo{pool: pool}
}

func videoCardDest(v *models.VideoCard) []any {
	return []any{&v.ID, &v.Title, &v.Description, &v.Thumbnail, &v.Visibility, &v.Status,
		&v.Views, &v.DurationSeconds, &v.CreatedAt, &v.UpdatedAt,
		&v.Channel.ID, &v.Channel.UserName, &v.Channel.ChannelName, &v.Channel.Avatar}
}

func collectCards(rows pgx.Rows) ([]*models.VideoCard, error) {
	defer rows.Close()
	videos := make([]*models.VideoCard, 0)
	for rows.Next() {
		var v models.VideoCard
		if err := rows.Scan(videoCardDest(&v)...); err != nil {
			return nil, err
		}
		videos = append(videos, &v)
	}
	return videos, rows.Err()
}

func (r *VideoCatalogRepo) ListPublished(ctx context.Context, f repository.VideoFilter, offset, limit int) ([]*models.VideoCard, error) {
	where := []string{publishedVideoCond}
	args := []any{}
	arg := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}
	orderBy := "v.created_at DESC"
	if f.IDs != nil {
		p := arg(f.IDs)
		where = append(where, "v.id = ANY("+p+"::uuid[])")
		orderBy = "array_position(" + p + "::uuid[], v.id)"
	}
	if f.ChannelID != nil {
		where = append(where, "v.user_id = "+arg(*f.ChannelID))
	}
	if f.SubscriberID != nil {
		where = append(where, "v.user_id IN (SELECT channel_id FROM subscriptions WHERE subscriber_id = "+arg(*f.SubscriberID)+")")
	}
	if f.LikedBy != nil {
		where = append(where, "v.id IN (SELECT video_id FROM video_reactions WHERE value = 1 AND user_id = "+arg(*f.LikedBy)+")")
	}
	if f.ExcludeID != nil {
		where = append(where, "v.id <> "+arg(*f.ExcludeID))
	}
	query := videoCardSelect + videoCardFrom +
		" WHERE " + strings.Join(where, " AND ") +
		" ORDER BY " + orderBy +
		" OFFSET " + arg(offset) + " LIMIT " + arg(limit)
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return collectCards(rows)
}

func (r *VideoCatalogRepo) ListByOwner(ctx context.Context, ownerID uuid.UUID, offset, limit int) ([]*models.VideoCard, error) {
	rows, err := r.pool.Query(ctx, videoCardSelect+videoCardFrom+`
		WHERE v.user_id = $1
		ORDER BY v.created_at DESC
		OFFSET $2 LIMIT $3`, ownerID, offset, limit)
	if err != nil {
		return nil, err
	}
	return collectCards(rows)
}

func (r *VideoCatalogRepo) GetDetails(ctx context.Context, id uuid.UUID, viewerID *uuid.UUID) (*models.VideoDetails, error) {
	var d models.VideoDetails
	dest := append(videoCardDest(&d.VideoCard),
		&d.ManifestPath, &d.AvailableQualities, &d.Likes, &d.Dislikes, &d.MyReaction, &d.CommentCount, &d.Subscribers, &d.IsSubscribed)
	err := r.pool.QueryRow(ctx, videoCardSelect+`,
			vm.manifest_path, COALESCE(vm.available_qualities, '{}'),
			(SELECT COUNT(*) FROM video_reactions vr WHERE vr.video_id = v.id AND vr.value = 1),
			(SELECT COUNT(*) FROM video_reactions vr WHERE vr.video_id = v.id AND vr.value = -1),
			COALESCE((SELECT vr.value FROM video_reactions vr WHERE vr.video_id = v.id AND vr.user_id = $2::uuid), 0),
			(SELECT COUNT(*) FROM comments c WHERE c.video_id = v.id),
			(SELECT COUNT(*) FROM subscriptions s WHERE s.channel_id = u.id),
			EXISTS (SELECT 1 FROM subscriptions s WHERE s.channel_id = u.id AND s.subscriber_id = $2::uuid)`+
		videoCardFrom+` WHERE v.id = $1`, id, viewerID).Scan(dest...)
	if err != nil {
		return nil, err
	}
	return &d, nil
}

func (r *VideoCatalogRepo) GetOwnerID(ctx context.Context, id uuid.UUID) (uuid.UUID, error) {
	var owner uuid.UUID
	err := r.pool.QueryRow(ctx, `SELECT user_id FROM videos WHERE id = $1`, id).Scan(&owner)
	return owner, err
}

func (r *VideoCatalogRepo) UpdateDetails(ctx context.Context, v *models.Video) error {
	// nil fields are left unchanged
	_, err := r.pool.Exec(ctx, `
		UPDATE videos SET
			title = COALESCE($2, title),
			description = COALESCE($3, description),
			visibility = COALESCE($4, visibility),
			thumbnail = COALESCE($5, thumbnail),
			updated_at = NOW()
		WHERE id = $1`, v.ID, v.Title, v.Description, v.Visibility, v.Thumbnail)
	return err
}

func (r *VideoCatalogRepo) Delete(ctx context.Context, id uuid.UUID) error {
	// video_metadata, reactions and comments cascade
	_, err := r.pool.Exec(ctx, `DELETE FROM videos WHERE id = $1`, id)
	return err
}

func (r *VideoCatalogRepo) IncrementViews(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `UPDATE videos SET views = views + 1 WHERE id = $1`, id)
	return err
}

func (r *VideoCatalogRepo) SetReaction(ctx context.Context, videoID, userID uuid.UUID, value int) error {
	if value == 0 {
		_, err := r.pool.Exec(ctx, `DELETE FROM video_reactions WHERE video_id = $1 AND user_id = $2`, videoID, userID)
		return err
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO video_reactions (video_id, user_id, value) VALUES ($1, $2, $3)
		ON CONFLICT (user_id, video_id) DO UPDATE SET value = EXCLUDED.value, created_at = NOW()`,
		videoID, userID, value)
	return err
}

type CommentRepo struct{ pool *pgxpool.Pool }

func NewCommentRepo(pool *pgxpool.Pool) *CommentRepo {
	return &CommentRepo{pool: pool}
}

const commentSelect = `
	SELECT c.id, c.video_id, c.content, c.created_at, c.updated_at, u.id, u.user_name, u.channel_name, u.avatar
	FROM comments c JOIN users u ON u.id = c.user_id`

func scanComment(row pgx.Row) (*models.Comment, error) {
	var c models.Comment
	if err := row.Scan(&c.ID, &c.VideoID, &c.Content, &c.CreatedAt, &c.UpdatedAt,
		&c.Author.ID, &c.Author.UserName, &c.Author.ChannelName, &c.Author.Avatar); err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *CommentRepo) List(ctx context.Context, videoID uuid.UUID, offset, limit int) ([]*models.Comment, error) {
	rows, err := r.pool.Query(ctx, commentSelect+` WHERE c.video_id = $1 ORDER BY c.created_at DESC OFFSET $2 LIMIT $3`,
		videoID, offset, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	comments := make([]*models.Comment, 0)
	for rows.Next() {
		c, err := scanComment(rows)
		if err != nil {
			return nil, err
		}
		comments = append(comments, c)
	}
	return comments, rows.Err()
}

func (r *CommentRepo) Get(ctx context.Context, id uuid.UUID) (*models.Comment, error) {
	return scanComment(r.pool.QueryRow(ctx, commentSelect+` WHERE c.id = $1`, id))
}

func (r *CommentRepo) Insert(ctx context.Context, id, videoID, userID uuid.UUID, content string) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO comments (id, video_id, user_id, content) VALUES ($1, $2, $3, $4)`,
		id, videoID, userID, content)
	return err
}

func (r *CommentRepo) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM comments WHERE id = $1`, id)
	return err
}
