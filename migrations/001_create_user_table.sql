-- +goose up

CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY,
	channel_name TEXT,
	user_name TEXT NOT NULL,
    subscribers INT DEFAULT 0,
	avatar TEXT,
	status TEXT CHECK (status IN ('active','inactive')) DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_users_user_name ON users (user_name);
CREATE INDEX IF NOT EXISTS idx_users_channel_name ON users (channel_name);

-- +goose down
DROP TABLE IF EXISTS users;