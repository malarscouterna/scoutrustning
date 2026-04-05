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
    AND (COALESCE(array_length(@statuses::text[], 1), 0) = 0 OR a.status = ANY(@statuses))
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
    status, individually_tracked, approval_level,
    description, instructions, purchase_date, purchase_price, place
) VALUES (
    @group_id, @commercial_name, @common_name, @category_id, @location_id,
    @status, @individually_tracked, @approval_level,
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
    approval_level = @approval_level,
    description = @description,
    instructions = @instructions,
    purchase_date = @purchase_date,
    purchase_price = @purchase_price,
    place = @place,
    updated_at = now()
WHERE id = @id AND group_id = @group_id
RETURNING *;

-- name: UpdateArticleStatus :one
UPDATE articles SET status = @status, expected_available_date = @expected_available_date, updated_at = now()
WHERE id = @id AND group_id = @group_id
RETURNING *;

-- name: DeleteArticle :exec
DELETE FROM articles
WHERE id = @id AND group_id = @group_id;

-- name: ListArticlesByUserBookings :many
-- Returns articles with given statuses that are linked to the user's bookings
-- (created by user or assigned to one of their units), or where the user
-- reported an issue (is an actor on an article event).
SELECT a.*,
    l.name AS location_name,
    c.name AS category_name
FROM articles a
JOIN locations l ON a.location_id = l.id
JOIN categories c ON a.category_id = c.id
WHERE a.group_id = @group_id
    AND (COALESCE(array_length(@statuses::text[], 1), 0) = 0 OR a.status = ANY(@statuses))
    AND (
        a.id IN (
            SELECT bi.article_id FROM booking_items bi
            JOIN bookings b ON bi.booking_id = b.id
            WHERE b.group_id = @group_id
                AND (b.created_by = @user_id OR b.used_by_unit_id = ANY(
                    SELECT un.id FROM units un WHERE un.group_id = @group_id AND un.name = ANY(@unit_names::text[])
                ))
        )
        OR a.id IN (
            SELECT ae.article_id FROM article_events ae
            WHERE ae.group_id = @group_id AND ae.actor_id = @user_id
        )
    )
ORDER BY c.sort_order, c.name, a.commercial_name, a.common_name;

-- name: ListArticlesWithAvailability :many
-- Returns articles enriched with current booking context for a given date.
SELECT a.*,
    l.name AS location_name,
    c.name AS category_name,
    cur_booking.id AS current_booking_id,
    COALESCE(cur_booking.status, '') AS current_booking_status,
    cur_booking.end_date AS current_booking_end_date,
    cur_unit.name AS current_booking_unit_name
FROM articles a
JOIN locations l ON a.location_id = l.id
JOIN categories c ON a.category_id = c.id
LEFT JOIN LATERAL (
    SELECT b.id, b.status, b.end_date, b.used_by_unit_id
    FROM booking_items bi
    JOIN bookings b ON bi.booking_id = b.id
    WHERE bi.article_id = a.id
        AND b.group_id = a.group_id
        AND b.status IN ('confirmed', 'approved', 'picked_up')
        AND b.start_date <= @as_of_date
        AND b.end_date >= @as_of_date
        AND (bi.return_status IS NULL OR bi.return_status IN ('pending', 'delayed'))
    ORDER BY b.start_date
    LIMIT 1
) cur_booking ON true
LEFT JOIN units cur_unit ON cur_booking.used_by_unit_id = cur_unit.id
WHERE a.group_id = @group_id
    AND (sqlc.narg('category_id')::uuid IS NULL OR a.category_id = sqlc.narg('category_id'))
    AND (sqlc.narg('location_id')::uuid IS NULL OR a.location_id = sqlc.narg('location_id'))
    AND (COALESCE(array_length(@statuses::text[], 1), 0) = 0 OR a.status = ANY(@statuses))
    AND (sqlc.narg('search')::text IS NULL OR a.common_name ILIKE '%' || sqlc.narg('search') || '%' OR a.commercial_name ILIKE '%' || sqlc.narg('search') || '%')
ORDER BY c.sort_order, c.name, a.commercial_name, a.common_name;

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
