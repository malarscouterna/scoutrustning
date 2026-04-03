-- name: CreateUnit :one
INSERT INTO units (group_id, name)
VALUES (@group_id, @name)
RETURNING *;
