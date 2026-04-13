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

-- name: DeleteTeam :exec
DELETE FROM teams
WHERE id = @id AND group_id = @group_id;

-- name: CountActiveBookingsForTeam :one
SELECT count(*) FROM bookings
WHERE used_by_team_id = @team_id AND group_id = @group_id
AND status NOT IN ('returned', 'cancelled');

-- name: ListTeamsByNames :many
SELECT * FROM teams
WHERE group_id = @group_id AND name = ANY(@names::text[]);
