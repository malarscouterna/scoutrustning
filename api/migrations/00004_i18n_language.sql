-- +goose Up
ALTER TABLE users ADD COLUMN language text;
ALTER TABLE group_settings ADD COLUMN default_language text NOT NULL DEFAULT 'sv';

-- +goose Down
ALTER TABLE users DROP COLUMN language;
ALTER TABLE group_settings DROP COLUMN default_language;
