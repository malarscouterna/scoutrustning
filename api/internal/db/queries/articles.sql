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
    description, instructions, purchase_date, purchase_price, place, manager_notes
) VALUES (
    @group_id, @commercial_name, @common_name, @category_id, @location_id,
    @status, @individually_tracked, @approval_level,
    @description, @instructions, @purchase_date, @purchase_price, @place, @manager_notes
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
    manager_notes = @manager_notes,
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

-- name: BulkUpdateArticleStatus :execrows
UPDATE articles SET status = @status, updated_at = now()
WHERE id = ANY(@ids::uuid[]) AND group_id = @group_id;

-- name: BulkUpdateArticleLocation :execrows
UPDATE articles SET location_id = @location_id, updated_at = now()
WHERE id = ANY(@ids::uuid[]) AND group_id = @group_id;

-- name: ListActiveBookingConflicts :many
-- Returns articles from the given list that are in active bookings (confirmed/approved/picked_up)
-- with booking details for conflict resolution.
SELECT DISTINCT a.id AS article_id, a.common_name AS article_name,
    b.id AS booking_id,
    b.start_date AS booking_start_date, b.end_date AS booking_end_date,
    COALESCE(u.name, b.used_by_external, '') AS booking_unit
FROM articles a
JOIN booking_items bi ON bi.article_id = a.id
JOIN bookings b ON bi.booking_id = b.id
LEFT JOIN units u ON b.used_by_unit_id = u.id
WHERE a.id = ANY(@ids::uuid[]) AND a.group_id = @group_id
    AND b.status IN ('confirmed', 'approved', 'picked_up')
    AND (bi.return_status IS NULL OR bi.return_status IN ('pending', 'delayed'));

-- name: FindReplacementArticle :one
-- Finds a replacement article with the same commercial_name + location, bookable status,
-- not in the given exclude list, and not in any overlapping active booking.
SELECT a.id
FROM articles a
WHERE a.group_id = @group_id
    AND a.commercial_name = @commercial_name
    AND a.location_id = @location_id
    AND a.status IN ('ok', 'reported_usable')
    AND a.id != ALL(@exclude_ids::uuid[])
    AND a.id NOT IN (
        SELECT bi.article_id FROM booking_items bi
        JOIN bookings b ON bi.booking_id = b.id
        WHERE b.group_id = @group_id
            AND b.status IN ('confirmed', 'approved', 'picked_up', 'submitted', 'draft')
            AND b.start_date <= @end_date
            AND b.end_date >= @start_date
            AND (bi.return_status IS NULL OR bi.return_status IN ('pending', 'delayed'))
    )
ORDER BY a.created_at
LIMIT 1;

-- name: SwapBookingItemArticleByArticle :execrows
-- Swap all booking_items referencing old_article_id in a specific booking to new_article_id.
UPDATE booking_items SET article_id = @new_article_id
WHERE article_id = @old_article_id AND booking_id = @booking_id AND group_id = @group_id;


-- name: GetArticleGroupInfo :one
-- Returns shared fields from the representative (oldest) article in a group.
SELECT a.*,
    l.name AS location_name,
    c.name AS category_name
FROM articles a
JOIN locations l ON a.location_id = l.id
JOIN categories c ON a.category_id = c.id
WHERE a.group_id = @group_id
    AND a.commercial_name = @commercial_name
    AND a.location_id = @location_id
ORDER BY a.created_at
LIMIT 1;

-- name: CountArticlesInGroup :one
-- Counts non-archived articles in a quantity tracked group.
SELECT COUNT(*) AS count
FROM articles a
WHERE a.group_id = @group_id
    AND a.commercial_name = @commercial_name
    AND a.location_id = @location_id
    AND a.status != 'archived';

-- name: ListNewestInGroup :many
-- Returns articles in a group ordered newest first, for archiving excess.
-- Excludes articles in active bookings.
SELECT a.id
FROM articles a
WHERE a.group_id = @group_id
    AND a.commercial_name = @commercial_name
    AND a.location_id = @location_id
    AND a.status != 'archived'
    AND a.id NOT IN (
        SELECT bi.article_id FROM booking_items bi
        JOIN bookings b ON bi.booking_id = b.id
        WHERE b.group_id = @group_id
            AND b.status IN ('confirmed', 'approved', 'picked_up', 'submitted', 'draft')
            AND (bi.return_status IS NULL OR bi.return_status IN ('pending', 'delayed'))
    )
ORDER BY a.created_at DESC;

-- name: UpdateArticleGroupFields :execrows
-- Updates shared fields for all articles in a quantity tracked group.
UPDATE articles SET
    commercial_name = @new_commercial_name,
    common_name = @new_common_name,
    category_id = @category_id,
    location_id = @new_location_id,
    approval_level = @approval_level,
    description = @description,
    instructions = @instructions,
    place = @place,
    manager_notes = @manager_notes,
    individually_tracked = @individually_tracked,
    updated_at = now()
WHERE group_id = @group_id
    AND commercial_name = @old_commercial_name
    AND location_id = @old_location_id;

-- name: PropagateSharedFields :execrows
-- After editing an individually tracked article, sync shared fields to siblings.
UPDATE articles SET
    description = @description,
    instructions = @instructions,
    manager_notes = @manager_notes,
    category_id = @category_id,
    updated_at = now()
WHERE group_id = @group_id
    AND commercial_name = @commercial_name
    AND id != @exclude_id;

-- name: BulkUpdateArticleApproval :execrows
UPDATE articles SET approval_level = @approval_level, updated_at = now()
WHERE id = ANY(@ids::uuid[]) AND group_id = @group_id;

-- name: UpdateArticleGroupImagePath :execrows
-- Sets image_path for all articles matching commercial_name + location_id in a group.
UPDATE articles SET image_path = @image_path, updated_at = now()
WHERE group_id = @group_id
    AND commercial_name = @commercial_name
    AND location_id = @location_id;

-- name: ClearArticleGroupImagePath :execrows
-- Clears image_path for all articles matching commercial_name + location_id in a group.
-- Returns the old image_path so the caller can delete the file.
UPDATE articles SET image_path = NULL, updated_at = now()
WHERE group_id = @group_id
    AND commercial_name = @commercial_name
    AND location_id = @location_id
    AND image_path IS NOT NULL;

-- name: GetArticleGroupImagePath :one
-- Returns the current image_path for an article group (for deletion).
SELECT image_path FROM articles
WHERE group_id = @group_id
    AND commercial_name = @commercial_name
    AND location_id = @location_id
    AND image_path IS NOT NULL
LIMIT 1;
