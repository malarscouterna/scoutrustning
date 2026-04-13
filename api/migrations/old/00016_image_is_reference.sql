-- +goose Up
ALTER TABLE product_images ADD COLUMN is_reference boolean NOT NULL DEFAULT false;

-- Backfill: original uploads have id = file_id, copies have id != file_id
UPDATE product_images SET is_reference = true WHERE id != file_id;

-- +goose Down
ALTER TABLE product_images DROP COLUMN is_reference;
