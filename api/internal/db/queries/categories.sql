-- name: ListCategories :many
SELECT * FROM categories
WHERE group_id = @group_id
ORDER BY sort_order, name;

-- name: GetCategory :one
SELECT * FROM categories
WHERE id = @id AND group_id = @group_id;

-- name: CreateCategory :one
INSERT INTO categories (group_id, name, parent_id, sort_order)
VALUES (@group_id, @name, @parent_id, @sort_order)
RETURNING *;

-- name: UpdateCategory :one
UPDATE categories SET name = @name, parent_id = @parent_id, sort_order = @sort_order
WHERE id = @id AND group_id = @group_id
RETURNING *;

-- name: DeleteCategory :exec
DELETE FROM categories
WHERE id = @id AND group_id = @group_id;

-- name: GetCategoryByName :one
SELECT * FROM categories
WHERE group_id = @group_id AND name = @name AND parent_id IS NOT DISTINCT FROM @parent_id;
