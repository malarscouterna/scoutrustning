-- name: GetGroupSettings :one
SELECT * FROM group_settings
WHERE group_id = @group_id;

-- name: UpsertGroupSettings :one
INSERT INTO group_settings (
    group_id, notification_email_from, smtp_key_encrypted, gchat_webhook_url,
    default_approval_level, default_access_unknown, default_access_troop,
    default_access_role, image_upload_role, booking_role, article_edit_role,
    issue_resolve_role, manager_notes_role, default_language
)
VALUES (
    @group_id, @notification_email_from, @smtp_key_encrypted, @gchat_webhook_url,
    @default_approval_level, @default_access_unknown, @default_access_troop,
    @default_access_role, @image_upload_role, @booking_role, @article_edit_role,
    @issue_resolve_role, @manager_notes_role, @default_language
)
ON CONFLICT (group_id) DO UPDATE SET
    notification_email_from = @notification_email_from,
    smtp_key_encrypted = @smtp_key_encrypted,
    gchat_webhook_url = @gchat_webhook_url,
    default_approval_level = @default_approval_level,
    default_access_unknown = @default_access_unknown,
    default_access_troop = @default_access_troop,
    default_access_role = @default_access_role,
    image_upload_role = @image_upload_role,
    booking_role = @booking_role,
    article_edit_role = @article_edit_role,
    issue_resolve_role = @issue_resolve_role,
    manager_notes_role = @manager_notes_role,
    default_language = @default_language,
    updated_at = now()
RETURNING *;

-- name: UpdateSmtpSettings :one
UPDATE group_settings SET
    notification_email_from = @notification_email_from,
    smtp_host = @smtp_host,
    smtp_port = @smtp_port,
    smtp_tls = @smtp_tls,
    smtp_user = @smtp_user,
    updated_at = now()
WHERE group_id = @group_id
RETURNING *;

-- name: CreateGroupSettingsDefaults :one
INSERT INTO group_settings (group_id)
VALUES (@group_id)
ON CONFLICT (group_id) DO NOTHING
RETURNING *;

-- name: GetGroupNotificationDefaults :one
SELECT notification_defaults FROM group_settings
WHERE group_id = @group_id;

-- name: SetGroupNotificationDefaults :exec
UPDATE group_settings SET notification_defaults = @notification_defaults, updated_at = now()
WHERE group_id = @group_id;

-- name: CountArticlesForLocation :one
SELECT count(*) FROM articles
WHERE group_id = @group_id AND location_id = @location_id;

-- name: CountArticlesForCategory :one
SELECT count(*) FROM articles
WHERE group_id = @group_id AND category_id = @category_id;
