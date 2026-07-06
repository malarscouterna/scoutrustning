-- name: UpsertUser :one
INSERT INTO users (id, group_id, name, email, picture, max_access_level, team_ids)
VALUES (@id, @group_id, @name, @email, @picture, @max_access_level, @team_ids::uuid[])
ON CONFLICT (id) DO UPDATE SET
    name = EXCLUDED.name,
    email = EXCLUDED.email,
    picture = EXCLUDED.picture,
    max_access_level = EXCLUDED.max_access_level,
    team_ids = EXCLUDED.team_ids,
    updated_at = now()
RETURNING id, group_id, name, email, active_group_id, created_at, updated_at, language, max_access_level, notification_prefs, team_ids, notification_email, picture;

-- name: ListUsersByGroup :many
SELECT id, name, email, max_access_level FROM users
WHERE group_id = @group_id
  AND (cardinality(@access_levels::text[]) = 0 OR max_access_level = ANY(@access_levels::text[]))
ORDER BY name;

-- name: GetUser :one
SELECT id, group_id, name, email, active_group_id, created_at, updated_at, language, max_access_level, notification_prefs, team_ids, notification_email, picture FROM users
WHERE id = @id AND group_id = @group_id;

-- name: GetUserTeamAffiliations :many
SELECT t.id, t.name, t.type, t.access_level
FROM teams t
JOIN users u ON t.id = ANY(u.team_ids)
WHERE u.id = @id AND u.group_id = @group_id AND t.group_id = @group_id
ORDER BY t.type, t.name;

-- name: GetUserOpenBookings :many
-- Bookings the user owns, or has participated in via a non-management action
-- (adding items, pickup, return) logged on article_events. Management actions
-- (submit/approve/reject) live on booking_events and are intentionally excluded.
SELECT DISTINCT b.id, b.status, b.start_date, b.end_date, b.used_by_team_id, b.used_by_external, b.notes,
    t.name AS team_name
FROM bookings b
LEFT JOIN teams t ON t.id = b.used_by_team_id
WHERE b.group_id = @group_id
  AND b.status = ANY(@statuses::text[])
  AND (
    b.created_by = @id
    OR EXISTS (
      SELECT 1 FROM article_events ae
      WHERE ae.group_id = @group_id
        AND ae.actor_id = @id
        AND ae.event_type IN ('booked', 'picked_up', 'returned')
        AND ae.metadata->>'booking_id' = b.id::text
    )
  )
ORDER BY b.start_date DESC;

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
