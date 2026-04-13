-- +goose Up
ALTER TABLE articles ADD COLUMN manager_notes text NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE articles DROP COLUMN IF EXISTS manager_notes;
