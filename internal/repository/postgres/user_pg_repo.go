package postgres

import (
	"context"
	"errors"

	"github.com/ak-ansari/mytube/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UserRepo struct{ pool *pgxpool.Pool }

func NewUserRepo(pool *pgxpool.Pool) *UserRepo {
	return &UserRepo{pool: pool}
}

const userColumns = `id, user_name, COALESCE(channel_name, ''), COALESCE(avatar, ''), COALESCE(status, 'active'),
	COALESCE(email, ''), COALESCE(description, ''), COALESCE(subscribers, 0)`

func scanUser(row pgx.Row) (*models.User, error) {
	var u models.User
	if err := row.Scan(&u.ID, &u.UserName, &u.ChannelName, &u.Avatar, &u.Status, &u.Email, &u.Description, &u.Subscribers); err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *UserRepo) Upsert(ctx context.Context, u *models.User) (*models.User, error) {
	// only writes when the user is new or their Keycloak identity changed
	user, err := scanUser(r.pool.QueryRow(ctx, `
		INSERT INTO users (id, user_name, channel_name, email, status)
		VALUES ($1, $2, $3, $4, 'active')
		ON CONFLICT (id) DO UPDATE SET user_name = EXCLUDED.user_name, email = EXCLUDED.email, updated_at = NOW()
		WHERE users.user_name IS DISTINCT FROM EXCLUDED.user_name OR users.email IS DISTINCT FROM EXCLUDED.email
		RETURNING `+userColumns, u.ID, u.UserName, u.ChannelName, u.Email))
	if errors.Is(err, pgx.ErrNoRows) {
		return scanUser(r.pool.QueryRow(ctx, `SELECT `+userColumns+` FROM users WHERE id = $1`, u.ID))
	}
	return user, err
}

const channelSelect = `
	SELECT u.id, u.user_name, u.channel_name, u.avatar, u.description, u.created_at,
		(SELECT COUNT(*) FROM subscriptions s WHERE s.channel_id = u.id),
		(SELECT COUNT(*) FROM videos v WHERE v.user_id = u.id AND ` + publishedVideoCond + `),
		EXISTS (SELECT 1 FROM subscriptions s WHERE s.channel_id = u.id AND s.subscriber_id = $2::uuid)
	FROM users u`

func scanChannel(row pgx.Row) (*models.Channel, error) {
	var c models.Channel
	if err := row.Scan(&c.ID, &c.UserName, &c.ChannelName, &c.Avatar, &c.Description, &c.CreatedAt,
		&c.Subscribers, &c.VideoCount, &c.IsSubscribed); err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *UserRepo) GetChannel(ctx context.Context, id uuid.UUID, viewerID *uuid.UUID) (*models.Channel, error) {
	return scanChannel(r.pool.QueryRow(ctx, channelSelect+` WHERE u.id = $1`, id, viewerID))
}

func (r *UserRepo) GetProfile(ctx context.Context, id uuid.UUID) (*models.Profile, error) {
	c, err := r.GetChannel(ctx, id, nil)
	if err != nil {
		return nil, err
	}
	p := &models.Profile{Channel: *c}
	if err := r.pool.QueryRow(ctx, `SELECT email FROM users WHERE id = $1`, id).Scan(&p.Email); err != nil {
		return nil, err
	}
	return p, nil
}

func (r *UserRepo) UpdateProfile(ctx context.Context, id uuid.UUID, channelName, description, avatar *string) error {
	// nil fields are left unchanged
	_, err := r.pool.Exec(ctx, `
		UPDATE users SET
			channel_name = COALESCE($2, channel_name),
			description = COALESCE($3, description),
			avatar = COALESCE($4, avatar),
			updated_at = NOW()
		WHERE id = $1`, id, channelName, description, avatar)
	return err
}

func (r *UserRepo) Subscribe(ctx context.Context, subscriberID, channelID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO subscriptions (subscriber_id, channel_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		subscriberID, channelID)
	return err
}

func (r *UserRepo) Unsubscribe(ctx context.Context, subscriberID, channelID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM subscriptions WHERE subscriber_id = $1 AND channel_id = $2`, subscriberID, channelID)
	return err
}

func (r *UserRepo) ListSubscriptions(ctx context.Context, subscriberID uuid.UUID) ([]*models.Channel, error) {
	rows, err := r.pool.Query(ctx, channelSelect+`
		JOIN subscriptions sub ON sub.channel_id = u.id
		WHERE sub.subscriber_id = $1
		ORDER BY sub.created_at DESC`, subscriberID, subscriberID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	channels := make([]*models.Channel, 0)
	for rows.Next() {
		c, err := scanChannel(rows)
		if err != nil {
			return nil, err
		}
		channels = append(channels, c)
	}
	return channels, rows.Err()
}
