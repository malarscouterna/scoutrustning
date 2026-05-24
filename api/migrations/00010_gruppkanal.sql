-- +goose Up

-- Phase 3.6: Gruppkanal + personal_email_policy
-- Replaces individual_notifications_enabled with nullable gruppkanal_channels.
-- notification_prefs JSONB shape changes: per-channel booleans replaced by
-- a single "gruppkanal" boolean per event, alongside "personal_email_policy".

-- teams: nullable channels array (NULL = inherit group default, '{}' = explicit opt-out)
ALTER TABLE teams ADD COLUMN IF NOT EXISTS gruppkanal_channels text[];
ALTER TABLE teams DROP COLUMN IF EXISTS individual_notifications_enabled;

-- group_settings: default Gruppkanal composition for teams with NULL gruppkanal_channels
ALTER TABLE group_settings ADD COLUMN IF NOT EXISTS default_gruppkanal_channels text[] NOT NULL DEFAULT '{}';

-- +goose Down
-- Down migration is a no-op: individual_notifications_enabled was dropped and
-- restoring it alongside the new gruppkanal_channels column is not warranted.
