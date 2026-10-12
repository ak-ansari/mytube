-- +goose Up
CREATE TABLE
    IF NOT EXISTS videos (
        id UUID PRIMARY KEY,
        user_id UUID NOT NULL, -- uploader
        file_key TEXT NOT NULL, -- cloud storage path
        thumbnail TEXT, -- thumbnail path
        title TEXT, -- title of video
        description TEXT, -- description about video
        visibility TEXT CHECK (visibility IN ('public', 'private', 'unlisted')),
        stage INT DEFAULT 1,
        status TEXT DEFAULT 'upload_pending' CHECK (
            status IN (
                'upload_pending',
                'uploaded',
                'valid',
                'thumbnail_generated',
                'transcoded',
                'segment_generated',
                'checksum',
                'published',
                'done',
                'phase_1_failed',
                'phase_2_failed'
            )
        ),
        created_at TIMESTAMPTZ NOT NULL DEFAULT NOW (),
        updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW (),
        CONSTRAINT fk_user FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
    );

CREATE INDEX IF NOT EXISTS idx_videos_user_id ON videos (user_id);

CREATE INDEX IF NOT EXISTS idx_videos_status ON videos (status);

CREATE INDEX IF NOT EXISTS idx_videos_file_key ON videos (file_key);

CREATE INDEX IF NOT EXISTS idx_videos_title ON videos (title);

CREATE INDEX IF NOT EXISTS idx_videos_description ON videos (description);

CREATE INDEX IF NOT EXISTS idx_videos_visibility ON videos (visibility);

-- +goose Down
DROP TABLE IF EXISTS videos;