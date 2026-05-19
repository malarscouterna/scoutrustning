-- +goose Up

-- Allow users to override the ScoutID email address used for personal notifications.
-- NULL means use users.email (from ScoutID) as before.
ALTER TABLE users ADD COLUMN IF NOT EXISTS notification_email text;

-- +goose Down
ALTER TABLE users DROP COLUMN IF EXISTS notification_email;
