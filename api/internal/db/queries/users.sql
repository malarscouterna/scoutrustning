-- name: UpsertUser :one
INSERT INTO users (id, group_id, name, email, max_access_level)
VALUES (@id, @group_id, @name, @email, @max_access_level)
ON CONFLICT (id) DO UPDATE SET
    name = EXCLUDED.name,
    email = EXCLUDED.email,
    max_access_level = EXCLUDED.max_access_level,
    updated_at = now()
RETURNING *;

-- name: ListUsersByGroup :many
SELECT id, name, email, max_access_level FROM users
WHERE group_id = @group_id
  AND (cardinality(@access_levels::text[]) = 0 OR max_access_level = ANY(@access_levels::text[]))
ORDER BY name;

-- name: GetUser :one
SELECT * FROM users
WHERE id = @id AND group_id = @group_id;

-- name: UpdateUserLanguage :exec
UPDATE users SET language = @language, updated_at = now()
WHERE id = @id;

-- name: GetUserNotificationPrefs :one
SELECT notification_prefs FROM users
WHERE id = @id AND group_id = @group_id;

-- name: SetUserNotificationPrefs :exec
UPDATE users SET notification_prefs = @notification_prefs, updated_at = now()
WHERE id = @id AND group_id = @group_id;

-- name: ClearUserNotificationPrefs :exec
UPDATE users SET notification_prefs = '{}', updated_at = now()
WHERE id = @id AND group_id = @group_id;
