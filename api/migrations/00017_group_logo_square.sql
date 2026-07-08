-- +goose Up

-- Square logo/icon variant, distinct from the wide logo (logo_file_id).
-- Used on narrow/mobile displays where a wide banner logo would be squeezed
-- illegibly small; falls back to the wide logo when not set.
-- WebP stored at {imageDir}/logos/{logo_square_file_id}.webp (web display only - no email use yet).
ALTER TABLE group_settings ADD COLUMN logo_square_file_id uuid;

-- +goose Down
ALTER TABLE group_settings DROP COLUMN IF EXISTS logo_square_file_id;
