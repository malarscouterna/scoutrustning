-- +goose Up

-- Team-level notification settings for the broadcast/personal dispatch model.
ALTER TABLE teams ADD COLUMN notification_email               text;
ALTER TABLE teams ADD COLUMN notification_prefs               jsonb    NOT NULL DEFAULT '{}';
ALTER TABLE teams ADD COLUMN individual_notifications_enabled boolean  NOT NULL DEFAULT true;

-- enabled_channels replaces the hard-wired ["email"] slice in the handler.
-- Default '{email}' keeps existing groups working without any migration steps.
ALTER TABLE group_settings ADD COLUMN enabled_channels text[] NOT NULL DEFAULT '{email}';

-- Threading support: store the logical thread key and the email Message-ID
-- so follow-up notifications can set In-Reply-To headers.
ALTER TABLE notification_log ADD COLUMN thread_key  text;
ALTER TABLE notification_log ADD COLUMN message_id  text;

-- Broadcast log rows are not tied to a single user (they go to a team shared address).
-- Drop the FK so we can use a sentinel user_id of 'broadcast:<team_id>' for these rows.
ALTER TABLE notification_log DROP CONSTRAINT IF EXISTS notification_log_user_id_fkey;

-- +goose Down
ALTER TABLE teams DROP COLUMN IF EXISTS notification_email;
ALTER TABLE teams DROP COLUMN IF EXISTS notification_prefs;
ALTER TABLE teams DROP COLUMN IF EXISTS individual_notifications_enabled;
ALTER TABLE group_settings DROP COLUMN IF EXISTS enabled_channels;
ALTER TABLE notification_log DROP COLUMN IF EXISTS thread_key;
ALTER TABLE notification_log DROP COLUMN IF EXISTS message_id;
