-- +goose Up
-- videos.updated_at only moved on studio edits, so pipeline progress looked like inactivity
-- (the frontend's stale-upload check and draft cleanup read it). Bump it whenever the stage or status changes.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION videos_touch_updated_at() RETURNS trigger AS $$
BEGIN
    NEW.updated_at := NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

DROP TRIGGER IF EXISTS trg_videos_touch_updated_at ON videos;
CREATE TRIGGER trg_videos_touch_updated_at
    BEFORE UPDATE OF stage, status ON videos
    FOR EACH ROW
    WHEN (OLD.stage IS DISTINCT FROM NEW.stage OR OLD.status IS DISTINCT FROM NEW.status)
    EXECUTE FUNCTION videos_touch_updated_at();

-- +goose Down
DROP TRIGGER IF EXISTS trg_videos_touch_updated_at ON videos;
DROP FUNCTION IF EXISTS videos_touch_updated_at();
