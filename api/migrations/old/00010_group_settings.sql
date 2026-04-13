-- +goose Up
CREATE TABLE group_settings (
    group_id text PRIMARY KEY REFERENCES groups(id),
    notification_email_from text NOT NULL DEFAULT '',
    smtp_key_encrypted bytea,
    gchat_webhook_url text NOT NULL DEFAULT '',
    default_approval_level text NOT NULL DEFAULT 'none',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT group_settings_approval_check CHECK (default_approval_level IN ('none', 'low', 'high'))
);

ALTER TABLE articles ADD COLUMN import_batch_id uuid;
CREATE INDEX idx_articles_import_batch ON articles(import_batch_id) WHERE import_batch_id IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_articles_import_batch;
ALTER TABLE articles DROP COLUMN IF EXISTS import_batch_id;
DROP TABLE IF EXISTS group_settings;
