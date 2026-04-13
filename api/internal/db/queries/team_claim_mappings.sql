-- name: CreateTeamClaimMapping :one
INSERT INTO team_claim_mappings (group_id, team_id, claim_scope, claim_id)
VALUES (@group_id, @team_id, @claim_scope, @claim_id)
ON CONFLICT (group_id, claim_scope, claim_id) DO NOTHING
RETURNING *;

-- name: ListTeamClaimMappings :many
SELECT tcm.*, t.name AS team_name, t.type AS team_type, t.access_level
FROM team_claim_mappings tcm
JOIN teams t ON t.id = tcm.team_id
WHERE tcm.group_id = @group_id
ORDER BY t.name;

-- name: DeleteTeamClaimMapping :exec
DELETE FROM team_claim_mappings
WHERE id = @id AND group_id = @group_id;

-- name: GetTeamClaimMappingsByClaims :many
SELECT tcm.claim_scope, tcm.claim_id, t.id AS team_id, t.name AS team_name, t.type AS team_type, t.access_level
FROM team_claim_mappings tcm
JOIN teams t ON t.id = tcm.team_id
WHERE tcm.group_id = @group_id;
