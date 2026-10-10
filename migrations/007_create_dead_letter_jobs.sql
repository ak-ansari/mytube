-- +goose Up
-- Pipeline work that failed for good: a stage handler with a permanent error or out of retries,
-- or a bucket event that could not be turned into a video. Kept for inspection; nothing reads it back.
-- No FK on video_id: the log should survive the video being deleted.
CREATE TABLE IF NOT EXISTS dead_letter_jobs (
    id UUID PRIMARY KEY,
    source TEXT NOT NULL CHECK (source IN ('job', 'bucket_event')),
    video_id UUID,
    stage INT,
    attempts INT NOT NULL,
    retryable BOOLEAN NOT NULL, -- true: retries ran out; false: the error was permanent
    error TEXT NOT NULL,
    payload TEXT NOT NULL,      -- the raw job payload or bucket event
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_dead_letter_jobs_video_id ON dead_letter_jobs (video_id);
CREATE INDEX IF NOT EXISTS idx_dead_letter_jobs_created_at ON dead_letter_jobs (created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS dead_letter_jobs;
