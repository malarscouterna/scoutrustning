-- name: ListLocations :many
SELECT * FROM locations
WHERE group_id = @group_id
ORDER BY sort_order, name;

-- name: GetLocation :one
SELECT * FROM locations
WHERE id = @id AND group_id = @group_id;

-- name: CreateLocation :one
INSERT INTO locations (group_id, name, sort_order)
VALUES (@group_id, @name, @sort_order)
RETURNING *;

-- name: UpdateLocation :one
UPDATE locations SET name = @name, sort_order = @sort_order
WHERE id = @id AND group_id = @group_id
RETURNING *;

-- name: DeleteLocation :exec
DELETE FROM locations
WHERE id = @id AND group_id = @group_id;

-- name: GetLocationByName :one
SELECT * FROM locations
WHERE group_id = @group_id AND name = @name;
