-- name: UpsertUser :one
INSERT INTO users (id, group_id, name, email)
VALUES (@id, @group_id, @name, @email)
ON CONFLICT (id) DO UPDATE SET
    name = EXCLUDED.name,
    email = EXCLUDED.email,
    updated_at = now()
RETURNING *;

-- name: GetUser :one
SELECT * FROM users
WHERE id = @id AND group_id = @group_id;

-- name: UpdateUserLanguage :exec
UPDATE users SET language = @language, updated_at = now()
WHERE id = @id;
