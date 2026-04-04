-- name: CreateUnit :one
INSERT INTO units (group_id, name, type)
VALUES (@group_id, @name, @type)
RETURNING *;
