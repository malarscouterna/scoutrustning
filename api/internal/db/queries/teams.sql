-- name: CreateTeam :one
INSERT INTO teams (group_id, name, type, access_level)
VALUES (@group_id, @name, @type, @access_level)
RETURNING *;

-- name: ListTeams :many
SELECT t.*,
    COALESCE(
        (SELECT jsonb_agg(jsonb_build_object('claim_scope', tcm.claim_scope, 'claim_id', tcm.claim_id))
         FROM team_claim_mappings tcm WHERE tcm.team_id = t.id),
        '[]'::jsonb
    ) AS claim_mappings
FROM teams t
WHERE t.group_id = @group_id
ORDER BY t.name;

-- name: GetTeam :one
SELECT * FROM teams
WHERE id = @id AND group_id = @group_id;

-- name: GetTeamByName :one
SELECT * FROM teams
WHERE group_id = @group_id AND name = @name
LIMIT 1;

-- name: UpdateTeam :one
UPDATE teams SET
    name = @name,
    type = @type,
    access_level = @access_level
WHERE id = @id AND group_id = @group_id
RETURNING *;

-- name: UpdateTeamNotificationSettings :one
UPDATE teams SET
    notification_email = @notification_email,
    notification_prefs = @notification_prefs,
    individual_notifications_enabled = @individual_notifications_enabled
WHERE id = @id AND group_id = @group_id
RETURNING *;

-- name: GetTeamNotificationSettings :one
SELECT notification_email, notification_prefs, individual_notifications_enabled, gchat_space_id
FROM teams
WHERE id = @id AND group_id = @group_id;

-- name: UpdateTeamName :one
UPDATE teams SET name = @name
WHERE id = @id AND group_id = @group_id
RETURNING *;

-- name: IsTeamMember :one
SELECT EXISTS(
    SELECT 1 FROM users
    WHERE id = @user_id AND group_id = @group_id AND @team_id::uuid = ANY(team_ids)
) AS is_member;

-- name: DeleteTeam :exec
DELETE FROM teams
WHERE id = @id AND group_id = @group_id;

-- name: CountActiveBookingsForTeam :one
SELECT count(*) FROM bookings
WHERE used_by_team_id = @team_id AND group_id = @group_id
AND status NOT IN ('returned', 'cancelled');

-- name: CountManagerTeams :one
SELECT count(*) FROM teams
WHERE group_id = @group_id AND access_level = 'manager';

-- name: ListTeamsByNames :many
SELECT * FROM teams
WHERE group_id = @group_id AND name = ANY(@names::text[]);

-- name: SetTeamGchatSpace :exec
UPDATE teams SET gchat_space_id = @gchat_space_id
WHERE id = @id AND group_id = @group_id;

-- name: ClearTeamGchatSpace :exec
UPDATE teams SET gchat_space_id = NULL
WHERE id = @id AND group_id = @group_id;

-- name: ListTeamsWithGchatInfo :many
SELECT id, name, gchat_space_id FROM teams
WHERE group_id = @group_id
ORDER BY name;
