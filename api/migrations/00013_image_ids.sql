-- +goose Up

ALTER TABLE articles ADD COLUMN image_ids jsonb NOT NULL DEFAULT '[]';

-- Migrate existing image_path into single-element arrays
UPDATE articles SET image_ids = jsonb_build_array(image_path)
WHERE image_path IS NOT NULL AND image_path != '';

ALTER TABLE articles DROP COLUMN image_path;

-- +goose Down

ALTER TABLE articles ADD COLUMN image_path text;

-- Restore first image from array
UPDATE articles SET image_path = image_ids->>0
WHERE jsonb_array_length(image_ids) > 0;

ALTER TABLE articles DROP COLUMN image_ids;
