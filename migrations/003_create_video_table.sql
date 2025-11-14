-- +goose Up
CREATE TABLE IF NOT EXISTS videos (
    id UUID PRIMARY KEY,
    video_id UUID NOT NULL,                  -- reference to video_metadata table
    user_id UUID NOT NULL,                   -- uploader
    file_key TEXT NOT NULL,                  -- cloud storage path
    thumbnail TEXT,                          -- thumbnail path
    title TEXT,                     -- title of video
    description TEXT,                        -- description about video
    visibility TEXT CHECK (visibility IN ('public', 'private', 'unlisted')),
    status TEXT NOT NULL CHECK (status IN ('uploading', 'uploaded', 'valid', 'processing', 'ready', 'failed')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT fk_video
        FOREIGN KEY (video_id)
        REFERENCES video_metadata(id)
        ON DELETE CASCADE,

    CONSTRAINT fk_user
        FOREIGN KEY (user_id)
        REFERENCES users(id)
        ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_videos_user_id ON videos (user_id);
CREATE INDEX IF NOT EXISTS idx_videos_status ON videos (status);
CREATE INDEX IF NOT EXISTS idx_videos_file_key ON videos (file_key);
CREATE INDEX IF NOT EXISTS idx_videos_title ON videos (title);
CREATE INDEX IF NOT EXISTS idx_videos_description ON videos (description);
CREATE INDEX IF NOT EXISTS idx_videos_visibility ON videos (visibility);
CREATE INDEX IF NOT EXISTS idx_videos_video_id ON videos (video_id);

-- +goose Down
DROP TABLE IF EXISTS videos;
