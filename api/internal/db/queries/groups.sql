-- name: CreateGroup :one
INSERT INTO groups (id, name, org_type, scoutnet_id)
VALUES (@id, @name, @org_type, @scoutnet_id)
ON CONFLICT (id) DO NOTHING
RETURNING *;

-- name: GetGroup :one
SELECT * FROM groups
WHERE id = @id;

-- name: UpdateGroupName :one
UPDATE groups SET name = @name
WHERE id = @id
RETURNING *;
