-- name: GetGroupSettings :one
SELECT * FROM group_settings
WHERE group_id = @group_id;

-- name: UpsertGroupSettings :one
INSERT INTO group_settings (group_id, notification_email_from, smtp_key_encrypted, gchat_webhook_url, default_approval_level)
VALUES (@group_id, @notification_email_from, @smtp_key_encrypted, @gchat_webhook_url, @default_approval_level)
ON CONFLICT (group_id) DO UPDATE SET
    notification_email_from = @notification_email_from,
    smtp_key_encrypted = @smtp_key_encrypted,
    gchat_webhook_url = @gchat_webhook_url,
    default_approval_level = @default_approval_level,
    updated_at = now()
RETURNING *;

-- name: CountArticlesForLocation :one
SELECT count(*) FROM articles
WHERE group_id = @group_id AND location_id = @location_id;

-- name: CountArticlesForCategory :one
SELECT count(*) FROM articles
WHERE group_id = @group_id AND category_id = @category_id;
