-- +goose Up

-- Phase 3.5b: Google Chat integration
-- Adds service account credentials and space mapping; drops stale columns.

-- group_settings: add service account storage and admin email
ALTER TABLE group_settings
    ADD COLUMN IF NOT EXISTS gchat_service_account_json_encrypted bytea,
    ADD COLUMN IF NOT EXISTS gchat_admin_email text NOT NULL DEFAULT '';

-- Drop old webhook URL columns (never populated in production)
ALTER TABLE group_settings DROP COLUMN IF EXISTS gchat_webhook_url;
ALTER TABLE teams          DROP COLUMN IF EXISTS gchat_webhook_url;
ALTER TABLE users          DROP COLUMN IF EXISTS gchat_webhook_url;

-- Drop superseded notification_channel column (replaced by notification_prefs jsonb)
ALTER TABLE users DROP COLUMN IF EXISTS notification_channel;

-- teams: add Chat Space mapping
ALTER TABLE teams ADD COLUMN IF NOT EXISTS gchat_space_id text;

-- +goose Down
-- Down migration is a no-op: columns added here only exist in this branch and
-- restoring gchat_webhook_url (dropped above) is not warranted.
