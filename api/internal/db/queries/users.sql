-- name: UpsertUser :one
INSERT INTO users (id, group_id, name, email, max_access_level, team_ids)
VALUES (@id, @group_id, @name, @email, @max_access_level, @team_ids::uuid[])
ON CONFLICT (id, group_id) DO UPDATE SET
    name = EXCLUDED.name,
    email = EXCLUDED.email,
    max_access_level = EXCLUDED.max_access_level,
    team_ids = EXCLUDED.team_ids,
    updated_at = now()
RETURNING id, group_id, name, email, created_at, updated_at, language, max_access_level, notification_prefs, team_ids, notification_email;

-- name: ListUsersByGroup :many
SELECT id, name, email, max_access_level FROM users
WHERE group_id = @group_id
  AND (cardinality(@access_levels::text[]) = 0 OR max_access_level = ANY(@access_levels::text[]))
ORDER BY name;

-- name: GetUser :one
SELECT id, group_id, name, email, created_at, updated_at, language, max_access_level, notification_prefs, team_ids, notification_email FROM users
WHERE id = @id AND group_id = @group_id;

-- name: UpdateUserLanguage :exec
UPDATE users SET language = @language, updated_at = now()
WHERE id = @id AND group_id = @group_id;

-- name: GetUserNotificationPrefs :one
SELECT notification_prefs FROM users
WHERE id = @id AND group_id = @group_id;

-- name: SetUserNotificationPrefs :exec
UPDATE users SET notification_prefs = @notification_prefs, updated_at = now()
WHERE id = @id AND group_id = @group_id;

-- name: ClearUserNotificationPrefs :exec
UPDATE users SET notification_prefs = '{}', updated_at = now()
WHERE id = @id AND group_id = @group_id;

-- name: ResetAllNotificationPrefs :one
WITH updated AS (
  UPDATE users SET notification_prefs = '{}', updated_at = now()
  WHERE group_id = @group_id
  RETURNING id
)
SELECT count(*) AS reset_count FROM updated;

-- name: GetTeamMembersWithEmails :many
SELECT id, name, email, language, max_access_level, notification_prefs, notification_email FROM users
WHERE group_id = @group_id AND @team_id::uuid = ANY(team_ids)
ORDER BY name;

-- name: GetGroupManagers :many
SELECT id, name, email, language, max_access_level, notification_prefs, notification_email FROM users
WHERE group_id = @group_id AND max_access_level = 'manager'
ORDER BY name;

-- name: SetUserNotificationEmail :exec
UPDATE users SET notification_email = @notification_email, updated_at = now()
WHERE id = @id AND group_id = @group_id;

-- name: ClearUserNotificationEmail :exec
UPDATE users SET notification_email = NULL, updated_at = now()
WHERE id = @id AND group_id = @group_id;
