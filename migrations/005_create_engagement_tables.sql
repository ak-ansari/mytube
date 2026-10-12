-- +goose Up
ALTER TABLE videos ADD COLUMN IF NOT EXISTS views BIGINT NOT NULL DEFAULT 0;
CREATE INDEX IF NOT EXISTS idx_videos_created_at ON videos (created_at DESC);

-- one row per (user, video); value 1 = like, -1 = dislike
CREATE TABLE IF NOT EXISTS video_reactions (
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    video_id UUID NOT NULL REFERENCES videos (id) ON DELETE CASCADE,
    value SMALLINT NOT NULL CHECK (value IN (-1, 1)),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, video_id)
);
CREATE INDEX IF NOT EXISTS idx_video_reactions_video_id ON video_reactions (video_id);

CREATE TABLE IF NOT EXISTS comments (
    id UUID PRIMARY KEY,
    video_id UUID NOT NULL REFERENCES videos (id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    content TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_comments_video_id ON comments (video_id, created_at DESC);

CREATE TABLE IF NOT EXISTS subscriptions (
    subscriber_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    channel_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (subscriber_id, channel_id),
    CHECK (subscriber_id <> channel_id)
);
CREATE INDEX IF NOT EXISTS idx_subscriptions_channel_id ON subscriptions (channel_id);

-- +goose Down
DROP TABLE IF EXISTS subscriptions;
DROP TABLE IF EXISTS comments;
DROP TABLE IF EXISTS video_reactions;
DROP INDEX IF EXISTS idx_videos_created_at;
ALTER TABLE videos DROP COLUMN IF EXISTS views;
