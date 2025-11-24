-- +goose Up
CREATE TABLE IF NOT EXISTS video_metadata (
    id UUID PRIMARY KEY,
    video_id UUID NOT NULL, -- reference to videos table
    filename TEXT NOT NULL,
    original_object_key TEXT NOT NULL,
    sha256 TEXT,
    duration_seconds INT,
    size BIGINT NOT NULL,
    codec_video TEXT,
    codec_audio TEXT,
    width INT,
    height INT,
    manifest_path TEXT,
    available_qualities TEXT[] DEFAULT '{}',
    thumbnail TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT fk_video FOREIGN KEY (video_id) REFERENCES videos (id) ON DELETE CASCADE
);

-- Useful indexes
CREATE INDEX IF NOT EXISTS idx_video_metadata_filename ON video_metadata (filename);
CREATE INDEX IF NOT EXISTS idx_video_metadata_original_object_key ON video_metadata (original_object_key);

-- +goose Down
DROP TABLE IF EXISTS video_metadata;
