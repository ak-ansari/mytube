-- +goose Up
-- users are provisioned from Keycloak on first authenticated request; users.id = Keycloak "sub"
ALTER TABLE users ADD COLUMN IF NOT EXISTS email TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS description TEXT;

-- +goose Down
ALTER TABLE users DROP COLUMN IF EXISTS description;
ALTER TABLE users DROP COLUMN IF EXISTS email;
