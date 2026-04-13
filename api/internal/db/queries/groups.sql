-- name: CreateGroup :one
INSERT INTO groups (id, name)
VALUES (@id, @name)
ON CONFLICT (id) DO NOTHING
RETURNING *;

-- name: GetGroup :one
SELECT * FROM groups
WHERE id = @id;

-- name: UpdateGroupName :one
UPDATE groups SET name = @name
WHERE id = @id
RETURNING *;
