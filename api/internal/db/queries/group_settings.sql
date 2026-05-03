-- name: GetGroupSettings :one
SELECT * FROM group_settings
WHERE group_id = @group_id;

-- name: UpsertGroupSettings :one
INSERT INTO group_settings (
    group_id, notification_email_from, smtp_key_encrypted,
    default_approval_level, default_access_unknown, default_access_troop,
    default_access_role, image_upload_role, booking_role, article_edit_role,
    issue_resolve_role, manager_notes_role, default_language
)
VALUES (
    @group_id, @notification_email_from, @smtp_key_encrypted,
    @default_approval_level, @default_access_unknown, @default_access_troop,
    @default_access_role, @image_upload_role, @booking_role, @article_edit_role,
    @issue_resolve_role, @manager_notes_role, @default_language
)
ON CONFLICT (group_id) DO UPDATE SET
    notification_email_from = @notification_email_from,
    smtp_key_encrypted = @smtp_key_encrypted,
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
SELECT notification_defaults, default_gruppkanal_channels FROM group_settings
WHERE group_id = @group_id;

-- name: SetGroupNotificationDefaults :exec
UPDATE group_settings SET
    notification_defaults = @notification_defaults,
    default_gruppkanal_channels = @default_gruppkanal_channels,
    updated_at = now()
WHERE group_id = @group_id;

-- name: SetGroupLogo :exec
UPDATE group_settings SET logo_file_id = @logo_file_id, updated_at = now()
WHERE group_id = @group_id;

-- name: ClearGroupLogo :exec
UPDATE group_settings SET logo_file_id = NULL, updated_at = now()
WHERE group_id = @group_id;

-- name: GetGroupLogoFileID :one
SELECT logo_file_id FROM group_settings WHERE group_id = @group_id;

-- name: SetGchatCredentials :exec
UPDATE group_settings SET
    gchat_service_account_json_encrypted = @gchat_service_account_json_encrypted,
    gchat_admin_email = @gchat_admin_email,
    updated_at = now()
WHERE group_id = @group_id;

-- name: ClearGchatCredentials :exec
UPDATE group_settings SET
    gchat_service_account_json_encrypted = NULL,
    gchat_admin_email = '',
    updated_at = now()
WHERE group_id = @group_id;

-- name: GetGchatCredentials :one
SELECT gchat_service_account_json_encrypted, gchat_admin_email FROM group_settings
WHERE group_id = @group_id;

-- name: UpdateEnabledChannels :exec
UPDATE group_settings SET enabled_channels = @enabled_channels, updated_at = now()
WHERE group_id = @group_id;

-- name: ClearAllGchatSpacesForGroup :exec
UPDATE teams SET gchat_space_id = NULL
WHERE group_id = @group_id;

-- name: CountArticlesForLocation :one
SELECT count(*) FROM articles
WHERE group_id = @group_id AND location_id = @location_id;

-- name: CountArticlesForCategory :one
SELECT count(*) FROM articles
WHERE group_id = @group_id AND category_id = @category_id;
