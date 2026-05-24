-- +goose Up

-- Track each user's highest resolved access level (updated on every login).
-- Stored denormalized so group members can be queried and filtered by access level
-- without rejoining through OIDC claim mappings.
ALTER TABLE users ADD COLUMN max_access_level text NOT NULL DEFAULT 'book';

-- Per-user notification preferences: {"booking_confirmed": {"email": true}, ...}
-- Missing keys fall back to group default, then system default.
ALTER TABLE users ADD COLUMN notification_prefs jsonb NOT NULL DEFAULT '{}';

-- Per-group SMTP override (non-secret fields in plain text, key encrypted separately)
ALTER TABLE group_settings ADD COLUMN smtp_host text NOT NULL DEFAULT '';
ALTER TABLE group_settings ADD COLUMN smtp_port integer NOT NULL DEFAULT 587;
ALTER TABLE group_settings ADD COLUMN smtp_tls text NOT NULL DEFAULT 'starttls';
ALTER TABLE group_settings ADD COLUMN smtp_user text NOT NULL DEFAULT '';

-- Per-group notification defaults: same shape as notification_prefs.
-- Overrides system defaults for users who have not set their own preference.
ALTER TABLE group_settings ADD COLUMN notification_defaults jsonb NOT NULL DEFAULT '{}';

-- Deduplication log for scheduled notification jobs (reminders, overdue alerts).
-- Event-triggered sends do not use this table.
CREATE TABLE notification_log (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    group_id    text NOT NULL REFERENCES groups(id),
    user_id     text NOT NULL REFERENCES users(id),
    event_type  text NOT NULL,
    entity_id   uuid NOT NULL,
    channel     text NOT NULL,
    status      text NOT NULL CHECK (status IN ('sent', 'failed', 'skipped')),
    error       text,
    created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX ON notification_log (entity_id, event_type, user_id, channel);

-- +goose Down
DROP TABLE IF EXISTS notification_log;

ALTER TABLE group_settings DROP COLUMN IF EXISTS notification_defaults;
ALTER TABLE group_settings DROP COLUMN IF EXISTS smtp_user;
ALTER TABLE group_settings DROP COLUMN IF EXISTS smtp_tls;
ALTER TABLE group_settings DROP COLUMN IF EXISTS smtp_port;
ALTER TABLE group_settings DROP COLUMN IF EXISTS smtp_host;

ALTER TABLE users DROP COLUMN IF EXISTS notification_prefs;
ALTER TABLE users DROP COLUMN IF EXISTS max_access_level;
