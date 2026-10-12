-- +goose Up
-- btree indexes on free text fail inserts/updates once a value exceeds ~2.7KB (btree row limit);
-- search goes through Elasticsearch, so these were never needed.
DROP INDEX IF EXISTS idx_videos_title;
DROP INDEX IF EXISTS idx_videos_description;

-- exactly one metadata row per video (every repo method looks it up by video_id)
CREATE UNIQUE INDEX IF NOT EXISTS idx_video_metadata_video_id ON video_metadata (video_id);

-- +goose Down
DROP INDEX IF EXISTS idx_video_metadata_video_id;
CREATE INDEX IF NOT EXISTS idx_videos_description ON videos (description);
CREATE INDEX IF NOT EXISTS idx_videos_title ON videos (title);
