-- +goose Up

-- Store the masked display value alongside the encrypted blob so GET /group-settings
-- does not need to decrypt the key on every request.
ALTER TABLE group_settings ADD COLUMN IF NOT EXISTS smtp_key_masked text NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE group_settings DROP COLUMN IF EXISTS smtp_key_masked;
