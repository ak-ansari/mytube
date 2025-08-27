-- +goose Up
ALTER TABLE videos DROP COLUMN storage_backend;
-- +goose Down