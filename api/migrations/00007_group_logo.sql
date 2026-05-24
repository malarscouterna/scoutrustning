-- +goose Up

-- Logo image stored per group, used in email headers and the web header.
-- WebP stored at {imageDir}/logos/{logo_file_id}.webp (web display).
-- PNG stored at {imageDir}/logos/{logo_file_id}.png (email, universal client support).
ALTER TABLE group_settings ADD COLUMN logo_file_id uuid;

-- +goose Down
ALTER TABLE group_settings DROP COLUMN IF EXISTS logo_file_id;
