-- +goose Up
CREATE TABLE IF NOT EXISTS videos (
  id UUID PRIMARY KEY,
  filename TEXT NOT NULL,
  original_object_key TEXT NOT NULL,
  storage_backend TEXT NOT NULL,
  sha256 TEXT,
  duration_seconds INT,
  codec_video TEXT,
  codec_audio TEXT,
  width INT,
  height INT,
  status TEXT NOT NULL DEFAULT 'uploaded',
  available_qualities TEXT[] DEFAULT '{}',
  manifest_path TEXT,
  thumbnails JSONB,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS video_jobs (
  id BIGSERIAL PRIMARY KEY,
  video_id UUID NOT NULL,
  step TEXT NOT NULL,
  attempts INT NOT NULL DEFAULT 0,
  max_attempts INT NOT NULL DEFAULT 5,
  last_error TEXT,
  payload JSONB,
  status TEXT NOT NULL DEFAULT 'queued',
  available_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
CREATE INDEX IF NOT EXISTS idx_video_jobs_status ON video_jobs(status);
CREATE INDEX IF NOT EXISTS idx_video_jobs_available_at ON video_jobs(available_at);
CREATE INDEX IF NOT EXISTS idx_video_jobs_video_id ON video_jobs(video_id);