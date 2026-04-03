-- name: ListArticles :many
SELECT a.*,
    l.name AS location_name,
    c.name AS category_name
FROM articles a
JOIN locations l ON a.location_id = l.id
JOIN categories c ON a.category_id = c.id
WHERE a.group_id = @group_id
    AND (sqlc.narg('category_id')::uuid IS NULL OR a.category_id = sqlc.narg('category_id'))
    AND (sqlc.narg('location_id')::uuid IS NULL OR a.location_id = sqlc.narg('location_id'))
    AND (sqlc.narg('status')::text IS NULL OR a.status = sqlc.narg('status'))
    AND (sqlc.narg('search')::text IS NULL OR a.common_name ILIKE '%' || sqlc.narg('search') || '%' OR a.commercial_name ILIKE '%' || sqlc.narg('search') || '%')
ORDER BY c.sort_order, c.name, a.commercial_name, a.common_name;

-- name: GetArticle :one
SELECT a.*,
    l.name AS location_name,
    c.name AS category_name
FROM articles a
JOIN locations l ON a.location_id = l.id
JOIN categories c ON a.category_id = c.id
WHERE a.id = @id AND a.group_id = @group_id;

-- name: CreateArticle :one
INSERT INTO articles (
    group_id, commercial_name, common_name, category_id, location_id,
    status, individually_tracked, requires_approval,
    description, instructions, purchase_date, purchase_price, place
) VALUES (
    @group_id, @commercial_name, @common_name, @category_id, @location_id,
    @status, @individually_tracked, @requires_approval,
    @description, @instructions, @purchase_date, @purchase_price, @place
)
RETURNING *;

-- name: UpdateArticle :one
UPDATE articles SET
    commercial_name = @commercial_name,
    common_name = @common_name,
    category_id = @category_id,
    location_id = @location_id,
    status = @status,
    individually_tracked = @individually_tracked,
    requires_approval = @requires_approval,
    description = @description,
    instructions = @instructions,
    purchase_date = @purchase_date,
    purchase_price = @purchase_price,
    place = @place,
    updated_at = now()
WHERE id = @id AND group_id = @group_id
RETURNING *;

-- name: DeleteArticle :exec
DELETE FROM articles
WHERE id = @id AND group_id = @group_id;

-- name: CountArticlesByLocation :many
SELECT l.id, l.name, COUNT(a.id) AS count
FROM locations l
LEFT JOIN articles a ON a.location_id = l.id AND a.group_id = l.group_id
WHERE l.group_id = @group_id
GROUP BY l.id, l.name
ORDER BY l.sort_order;

-- name: CountArticlesByCategory :many
SELECT c.id, c.name, COUNT(a.id) AS count
FROM categories c
LEFT JOIN articles a ON a.category_id = c.id AND a.group_id = c.group_id
WHERE c.group_id = @group_id
GROUP BY c.id, c.name
ORDER BY c.sort_order;
