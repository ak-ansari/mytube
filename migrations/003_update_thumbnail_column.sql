-- +goose Up
ALTER TABLE videos RENAME COLUMN thumbnails TO thumbnail;

ALTER TABLE videos 
  ALTER COLUMN thumbnail TYPE text;
-- +goose Down