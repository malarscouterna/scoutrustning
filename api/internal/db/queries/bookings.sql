-- name: CreateBooking :one
INSERT INTO bookings (
    group_id, created_by, used_by_unit_id, used_by_external,
    used_by_external_contact, status, start_date, end_date, notes
) VALUES (
    @group_id, @created_by, @used_by_unit_id, @used_by_external,
    @used_by_external_contact, 'draft', @start_date, @end_date, @notes
)
RETURNING *;

-- name: GetBooking :one
SELECT b.*, u.name AS unit_name
FROM bookings b
LEFT JOIN units u ON b.used_by_unit_id = u.id
WHERE b.id = @id AND b.group_id = @group_id;

-- name: ListBookingsByUser :many
SELECT b.*, u.name AS unit_name
FROM bookings b
LEFT JOIN units u ON b.used_by_unit_id = u.id
WHERE b.group_id = @group_id
    AND (b.created_by = @user_id OR b.used_by_unit_id = ANY(
        SELECT un.id FROM units un WHERE un.group_id = @group_id AND un.name = ANY(@unit_names::text[])
    ))
ORDER BY b.created_at DESC;

-- name: ListAllBookings :many
SELECT b.*, u.name AS unit_name
FROM bookings b
LEFT JOIN units u ON b.used_by_unit_id = u.id
WHERE b.group_id = @group_id
ORDER BY b.created_at DESC;

-- name: ListBookingsByStatus :many
SELECT b.*, u.name AS unit_name
FROM bookings b
LEFT JOIN units u ON b.used_by_unit_id = u.id
WHERE b.group_id = @group_id AND b.status = @status
ORDER BY b.start_date;

-- name: UpdateBookingStatus :one
UPDATE bookings SET status = @status, updated_at = now()
WHERE id = @id AND group_id = @group_id
RETURNING *;

-- name: UpdateBooking :one
UPDATE bookings SET
    start_date = @start_date,
    end_date = @end_date,
    used_by_unit_id = @used_by_unit_id,
    used_by_external = @used_by_external,
    used_by_external_contact = @used_by_external_contact,
    notes = @notes,
    updated_at = now()
WHERE id = @id AND group_id = @group_id
RETURNING *;

-- name: GetUnitByID :one
SELECT * FROM units
WHERE id = @id AND group_id = @group_id;

-- name: AvailableArticlesExcludingBooking :many
-- Same as AvailableArticles but excludes items already in the given booking.
SELECT a.id, a.commercial_name, a.common_name, a.location_id,
    l.name AS location_name, a.place, a.status,
    a.individually_tracked, a.approval_level,
    a.expected_available_date
FROM articles a
JOIN locations l ON a.location_id = l.id
WHERE a.group_id = @group_id
    AND (
        a.status IN ('ok', 'reported_usable')
        OR (a.status = 'incoming' AND a.expected_available_date IS NOT NULL AND a.expected_available_date <= @start_date)
        OR (a.status = 'under_repair' AND a.expected_available_date IS NOT NULL AND a.expected_available_date <= @start_date)
    )
    AND a.id NOT IN (
        SELECT bi.article_id FROM booking_items bi
        JOIN bookings b ON bi.booking_id = b.id
        WHERE b.group_id = @group_id
            AND b.id != @exclude_booking_id
            AND b.status IN ('draft', 'confirmed', 'picked_up', 'submitted', 'approved')
            AND b.start_date <= @end_date
            AND b.end_date >= @start_date
            AND (bi.return_status IS NULL OR bi.return_status IN ('pending', 'delayed'))
    )
    AND a.id NOT IN (
        SELECT bi.article_id FROM booking_items bi
        WHERE bi.booking_id = @exclude_booking_id
    )
ORDER BY CASE a.status WHEN 'ok' THEN 0 WHEN 'incoming' THEN 1 WHEN 'under_repair' THEN 2 WHEN 'reported_usable' THEN 3 ELSE 4 END, a.commercial_name, a.common_name;

-- name: AddBookingItem :one
INSERT INTO booking_items (group_id, booking_id, article_id)
VALUES (@group_id, @booking_id, @article_id)
RETURNING *;

-- name: RemoveBookingItem :exec
DELETE FROM booking_items
WHERE id = @id AND group_id = @group_id AND booking_id = @booking_id;

-- name: ListBookingItems :many
SELECT bi.*,
    a.commercial_name,
    a.common_name,
    a.place,
    a.status AS article_status,
    a.expected_available_date AS article_expected_available_date,
    a.approval_level,
    a.individually_tracked,
    l.name AS location_name,
    c.name AS category_name
FROM booking_items bi
JOIN articles a ON bi.article_id = a.id
JOIN locations l ON a.location_id = l.id
JOIN categories c ON a.category_id = c.id
WHERE bi.booking_id = @booking_id AND bi.group_id = @group_id
ORDER BY c.name, a.commercial_name, a.common_name;

-- name: AvailableArticles :many
-- Returns articles that are bookable and not reserved by overlapping bookings.
SELECT a.id, a.commercial_name, a.common_name, a.category_id, a.location_id,
    l.name AS location_name, c.name AS category_name, a.place, a.status,
    a.individually_tracked, a.approval_level,
    a.expected_available_date
FROM articles a
JOIN locations l ON a.location_id = l.id
JOIN categories c ON a.category_id = c.id
WHERE a.group_id = @group_id
    AND (
        a.status IN ('ok', 'reported_usable')
        OR (a.status = 'incoming' AND a.expected_available_date IS NOT NULL AND a.expected_available_date <= @start_date)
        OR (a.status = 'under_repair' AND a.expected_available_date IS NOT NULL AND a.expected_available_date <= @start_date)
    )
    AND a.id NOT IN (
        SELECT bi.article_id FROM booking_items bi
        JOIN bookings b ON bi.booking_id = b.id
        WHERE b.group_id = @group_id
            AND b.status IN ('draft', 'confirmed', 'picked_up', 'submitted', 'approved')
            AND b.start_date <= @end_date
            AND b.end_date >= @start_date
            AND (bi.return_status IS NULL OR bi.return_status IN ('pending', 'delayed'))
    )
ORDER BY CASE a.status WHEN 'ok' THEN 0 WHEN 'incoming' THEN 1 WHEN 'under_repair' THEN 2 WHEN 'reported_usable' THEN 3 ELSE 4 END, a.commercial_name, a.common_name;

-- name: BookingMaxApprovalLevel :one
-- Returns the highest approval level across all articles in the booking.
SELECT COALESCE(
    (SELECT CASE
        WHEN EXISTS (SELECT 1 FROM booking_items bi JOIN articles a ON bi.article_id = a.id WHERE bi.booking_id = @booking_id AND bi.group_id = @group_id AND a.approval_level = 'high') THEN 'high'
        WHEN EXISTS (SELECT 1 FROM booking_items bi JOIN articles a ON bi.article_id = a.id WHERE bi.booking_id = @booking_id AND bi.group_id = @group_id AND a.approval_level = 'low') THEN 'low'
        ELSE 'none'
    END),
    'none'
) AS max_approval_level;

-- name: ApproveBooking :one
UPDATE bookings SET status = 'confirmed', updated_at = now()
WHERE id = @id AND group_id = @group_id AND status = 'submitted'
RETURNING *;

-- name: RejectBooking :one
UPDATE bookings SET status = 'draft', updated_at = now()
WHERE id = @id AND group_id = @group_id AND status = 'submitted'
RETURNING *;

-- name: CreateBookingEvent :one
INSERT INTO booking_events (group_id, booking_id, actor_id, event_type, message, metadata)
VALUES (@group_id, @booking_id, @actor_id, @event_type, @message, @metadata)
RETURNING *;

-- name: ListBookingEvents :many
SELECT be.*, u.name AS actor_name
FROM booking_events be
JOIN users u ON be.actor_id = u.id
WHERE be.booking_id = @booking_id AND be.group_id = @group_id
ORDER BY be.created_at ASC;

-- name: DeleteBooking :exec
DELETE FROM bookings
WHERE id = @id AND group_id = @group_id AND status = 'draft';

-- name: ListUnits :many
SELECT * FROM units
WHERE group_id = @group_id
ORDER BY type, name;

-- name: UpdateBookingItemPickupStatus :one
UPDATE booking_items SET pickup_status = @pickup_status
WHERE id = @id AND group_id = @group_id AND booking_id = @booking_id
RETURNING *;

-- name: SwapBookingItemArticle :one
UPDATE booking_items SET article_id = @new_article_id, pickup_status = 'swapped'
WHERE id = @id AND group_id = @group_id AND booking_id = @booking_id
RETURNING *;

-- name: AllItemsPickedUp :one
-- Returns true if every item in the booking has a non-null pickup_status.
SELECT NOT EXISTS (
    SELECT 1 FROM booking_items
    WHERE booking_id = @booking_id AND group_id = @group_id
        AND pickup_status IS NULL
) AS all_picked_up;

-- name: UpdateBookingItemReturnStatus :one
UPDATE booking_items SET return_status = @return_status
WHERE id = @id AND group_id = @group_id AND booking_id = @booking_id
RETURNING *;

-- name: AllItemsReturned :one
-- Returns true if every picked-up item has a final return status.
-- Delayed items are NOT final — they must be resolved before completing.
-- Items that were never picked up are excluded.
SELECT NOT EXISTS (
    SELECT 1 FROM booking_items
    WHERE booking_id = @booking_id AND group_id = @group_id
        AND pickup_status IS NOT NULL AND pickup_status != 'lost'
        AND (return_status IS NULL OR return_status = 'delayed')
) AS all_returned;

-- name: CleanupStaleDrafts :exec
-- Delete draft bookings older than the given threshold.
DELETE FROM bookings
WHERE group_id = @group_id AND status = 'draft'
    AND created_at < @older_than;

-- name: CleanupEmptyDrafts :execrows
-- Delete draft bookings with zero items older than the given threshold (all groups).
DELETE FROM bookings
WHERE status = 'draft'
    AND created_at < @older_than
    AND NOT EXISTS (
        SELECT 1 FROM booking_items WHERE booking_id = bookings.id
    );

-- name: NoItemsPickedUp :one
-- Returns true if every item in the booking has a null pickup_status.
SELECT NOT EXISTS (
    SELECT 1 FROM booking_items
    WHERE booking_id = @booking_id AND group_id = @group_id
        AND pickup_status IS NOT NULL
) AS none_picked_up;

-- name: SetPrePickupStatus :exec
UPDATE bookings SET pre_pickup_status = @pre_pickup_status
WHERE id = @id AND group_id = @group_id;

-- name: GetPrePickupStatus :one
SELECT pre_pickup_status FROM bookings
WHERE id = @id AND group_id = @group_id;
