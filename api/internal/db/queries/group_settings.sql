-- name: GetGroupSettings :one
SELECT * FROM group_settings
WHERE group_id = @group_id;

-- name: UpsertGroupSettings :one
INSERT INTO group_settings (
    group_id, notification_email_from, smtp_key_encrypted, gchat_webhook_url,
    default_approval_level, default_access_unknown, default_access_troop,
    default_access_role, image_upload_role, booking_role, article_edit_role,
    issue_resolve_role, manager_notes_role
)
VALUES (
    @group_id, @notification_email_from, @smtp_key_encrypted, @gchat_webhook_url,
    @default_approval_level, @default_access_unknown, @default_access_troop,
    @default_access_role, @image_upload_role, @booking_role, @article_edit_role,
    @issue_resolve_role, @manager_notes_role
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
    updated_at = now()
RETURNING *;

-- name: CreateGroupSettingsDefaults :one
INSERT INTO group_settings (group_id)
VALUES (@group_id)
ON CONFLICT (group_id) DO NOTHING
RETURNING *;

-- name: CountArticlesForLocation :one
SELECT count(*) FROM articles
WHERE group_id = @group_id AND location_id = @location_id;

-- name: CountArticlesForCategory :one
SELECT count(*) FROM articles
WHERE group_id = @group_id AND category_id = @category_id;
