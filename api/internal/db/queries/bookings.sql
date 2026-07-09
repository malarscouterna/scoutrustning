-- name: GetAllBookingsStartingOn :many
-- Returns confirmed/picked_up bookings across all groups whose start_date equals the given date.
-- Used by the daily reminder scheduler.
SELECT id, group_id, created_by, used_by_team_id, start_date, end_date
FROM bookings
WHERE status IN ('confirmed', 'picked_up')
  AND start_date = @date;

-- name: GetAllOverdueBookings :many
-- Returns picked_up bookings across all groups whose end_date is before the given date.
-- Used by the daily overdue scheduler; deduplication is handled via notification_log.
SELECT id, group_id, created_by, used_by_team_id, start_date, end_date
FROM bookings
WHERE status = 'picked_up'
  AND end_date < @date;

-- name: CreateBooking :one
INSERT INTO bookings (
    group_id, created_by, used_by_team_id, used_by_external,
    used_by_external_contact, status, start_date, end_date, title
) VALUES (
    @group_id, @created_by, @used_by_team_id, @used_by_external,
    @used_by_external_contact, 'draft', @start_date, @end_date, @title
)
RETURNING *;

-- name: GetBooking :one
SELECT b.*, t.name AS team_name, u.name AS creator_name, u.picture AS creator_picture
FROM bookings b
LEFT JOIN teams t ON b.used_by_team_id = t.id
LEFT JOIN users u ON b.created_by = u.id
WHERE b.id = @id AND b.group_id = @group_id;

-- name: ListBookingsByUser :many
SELECT b.*, t.name AS team_name, u.name AS creator_name
FROM bookings b
LEFT JOIN teams t ON b.used_by_team_id = t.id
LEFT JOIN users u ON b.created_by = u.id
WHERE b.group_id = @group_id
    AND (b.created_by = @user_id OR b.used_by_team_id = ANY(
        SELECT tm.id FROM teams tm WHERE tm.group_id = @group_id AND tm.name = ANY(@team_names::text[])
    ))
ORDER BY b.created_at DESC;

-- name: ListAllBookings :many
SELECT b.*, t.name AS team_name, u.name AS creator_name
FROM bookings b
LEFT JOIN teams t ON b.used_by_team_id = t.id
LEFT JOIN users u ON b.created_by = u.id
WHERE b.group_id = @group_id
ORDER BY b.created_at DESC;

-- name: ListBookingsByStatus :many
SELECT b.*, t.name AS team_name, u.name AS creator_name
FROM bookings b
LEFT JOIN teams t ON b.used_by_team_id = t.id
LEFT JOIN users u ON b.created_by = u.id
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
    used_by_team_id = @used_by_team_id,
    used_by_external = @used_by_external,
    used_by_external_contact = @used_by_external_contact,
    title = @title,
    updated_at = now()
WHERE id = @id AND group_id = @group_id
RETURNING *;

-- name: GetTeamByID :one
SELECT * FROM teams
WHERE id = @id AND group_id = @group_id;

-- name: AvailableArticlesExcludingBooking :many
-- Availability for a booking's date range, excluding conflicts from other
-- overlapping bookings. When exclude_own_items is true, articles already
-- assigned to the given booking are also excluded from the result - used
-- when offering NEW items to add or swap into the booking. When false, the
-- booking's own current items are left in the result - used to revalidate
-- that a booking's existing items remain assignable after its dates change
-- (they must not appear as conflicting with themselves).
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
    AND (
        NOT @exclude_own_items::boolean
        OR a.id NOT IN (
            SELECT bi.article_id FROM booking_items bi
            WHERE bi.booking_id = @exclude_booking_id
        )
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
    a.description AS article_description,
    a.instructions AS article_instructions,
    a.status AS article_status,
    a.expected_available_date AS article_expected_available_date,
    a.approval_level,
    a.individually_tracked,
    a.image_ids,
    a.location_id,
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
    a.expected_available_date,
    a.image_ids, a.description, a.instructions
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
-- Stays 'rejected' (draft-like editable state) until the user starts editing
-- it (Update/AddItems/RemoveItem transition it to 'draft' at that point).
UPDATE bookings SET status = 'rejected', updated_at = now()
WHERE id = @id AND group_id = @group_id AND status = 'submitted'
RETURNING *;

-- name: CreateBookingEvent :one
INSERT INTO booking_events (group_id, booking_id, actor_id, event_type, message, metadata)
VALUES (@group_id, @booking_id, @actor_id, @event_type, @message, @metadata)
RETURNING *;

-- name: ListBookingEvents :many
SELECT be.*, u.name AS actor_name, u.picture AS actor_picture
FROM booking_events be
JOIN users u ON be.actor_id = u.id
WHERE be.booking_id = @booking_id AND be.group_id = @group_id
ORDER BY be.created_at ASC;

-- name: GetLatestBookingEvent :one
SELECT * FROM booking_events
WHERE booking_id = @booking_id AND group_id = @group_id
ORDER BY created_at DESC
LIMIT 1;

-- name: HasSubmittedEvent :one
-- Whether this booking has ever been submitted - drives whether item-change
-- events use pre-submission wording ("Påbörjade bokning") or add/remove delta
-- wording once it's been through the approval flow at least once.
SELECT EXISTS (
    SELECT 1 FROM booking_events
    WHERE booking_id = @booking_id AND group_id = @group_id AND event_type = 'submitted'
);

-- name: UpdateBookingEventMessage :one
-- Bumps created_at so the merged entry still sorts as the most recent activity.
UPDATE booking_events SET message = @message, metadata = @metadata, created_at = now()
WHERE id = @id AND group_id = @group_id
RETURNING *;

-- name: DeleteBooking :exec
DELETE FROM bookings
WHERE id = @id AND group_id = @group_id AND status = 'draft';

-- name: ListBookingTeams :many
SELECT id, group_id, name, type, access_level FROM teams
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
UPDATE booking_items SET return_status = @return_status, expected_return_date = @expected_return_date
WHERE id = @id AND group_id = @group_id AND booking_id = @booking_id
RETURNING *;

-- name: FindWaitingBookingItemsForArticle :many
-- Non-terminal bookings (docs/implementation/delayed-return-swap.md decision 2) already holding
-- the exact given article, whose own start_date has arrived by check_date - i.e.
-- bookings actively blocked by this article right now. Also doubles as the
-- "next expected user" preview query, called with check_date = the date typed
-- into the expected-return-date field before saving.
SELECT bi.id AS booking_item_id, bi.booking_id, b.start_date, b.end_date,
    b.created_by, u.name AS creator_name, u.picture AS creator_picture
FROM booking_items bi
JOIN bookings b ON bi.booking_id = b.id
LEFT JOIN users u ON b.created_by = u.id
WHERE bi.article_id = @article_id
    AND b.group_id = @group_id
    AND b.status IN ('draft', 'submitted', 'approved', 'confirmed')
    AND b.start_date <= @check_date
ORDER BY b.start_date ASC;

-- name: FindDelayedOrOverdueItems :many
-- Cross-group enumeration for the nightly swap-resolution job (mirrors
-- GetAllOverdueBookings's cross-group shape): booking_items still picked_up
-- where either someone already explicitly marked them delayed during return,
-- or the booking's end_date has passed @grace_cutoff (today minus the grace
-- period) with no return status recorded at all - an explicit "delayed" mark
-- is already a known problem and skips the grace period, but a booking that's
-- merely a day late with nobody flagging it yet shouldn't trigger a swap
-- before it's had a chance to come back on its own.
SELECT bi.id AS booking_item_id, bi.group_id, bi.article_id, bi.booking_id
FROM booking_items bi
JOIN bookings b ON bi.booking_id = b.id
WHERE b.status = 'picked_up'
    AND bi.pickup_status IS NOT NULL
    AND (
        bi.return_status = 'delayed'
        OR (bi.return_status IS NULL AND b.end_date < @grace_cutoff)
    );

-- name: GetBlockedItemsForBooking :many
-- Powers the booking-detail warning section (docs/implementation/delayed-return-swap.md): this
-- booking's own items whose start_date has arrived, where another booking still
-- holds the exact same article_id, picked_up and unresolved (delayed or simply
-- never returned) - i.e. this booking is actively blocked right now, mirroring
-- FindWaitingBookingItemsForArticle's "waiting" definition but starting from the
-- waiting booking instead of the article.
SELECT bi.id AS booking_item_id, a.commercial_name, a.common_name,
    holder_b.id AS holder_booking_id, holder_b.created_by AS holder_user_id,
    holder_u.name AS holder_name, holder_u.picture AS holder_picture,
    holder_bi.expected_return_date
FROM booking_items bi
JOIN bookings b ON bi.booking_id = b.id
JOIN articles a ON bi.article_id = a.id
JOIN booking_items holder_bi ON holder_bi.article_id = bi.article_id AND holder_bi.id != bi.id
JOIN bookings holder_b ON holder_bi.booking_id = holder_b.id
LEFT JOIN users holder_u ON holder_b.created_by = holder_u.id
WHERE b.id = @booking_id
    AND b.group_id = @group_id
    AND b.start_date <= CURRENT_DATE
    AND holder_b.status = 'picked_up'
    AND holder_bi.pickup_status IS NOT NULL
    AND (holder_bi.return_status IS NULL OR holder_bi.return_status = 'delayed')
ORDER BY bi.id;

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

-- name: GetBookingsNearingArchive :many
-- Bookings whose auto-archive deadline (docs/implementation/pre-release.md "Booking auto-archive setting")
-- falls between 23 and 24 hours from now (all groups). Called hourly (not the once-daily
-- scheduler) so the one-time advance warning lands close to a true 24h-before mark rather
-- than drifting by up to a full day between checks. Draft deadline runs from created_at,
-- not first-item-add, so the countdown (and this warning) applies from the moment a draft
-- is created, with or without items - this also supersedes the old separate 48h
-- empty-draft cleanup, which no longer exists.
SELECT b.*,
    (CASE WHEN b.status = 'draft' THEN b.created_at + (gs.draft_archive_days || ' days')::interval
         ELSE b.updated_at + (gs.rejected_archive_days || ' days')::interval END)::timestamptz AS archive_deadline
FROM bookings b
JOIN group_settings gs ON gs.group_id = b.group_id
WHERE (
    (b.status = 'draft' AND gs.draft_archive_days > 0
        AND b.created_at + (gs.draft_archive_days || ' days')::interval BETWEEN now() + interval '23 hours' AND now() + interval '24 hours')
    OR
    (b.status = 'rejected' AND gs.rejected_archive_days > 0
        AND b.updated_at + (gs.rejected_archive_days || ' days')::interval BETWEEN now() + interval '23 hours' AND now() + interval '24 hours')
);

-- name: GetBookingsPastArchiveDeadline :many
-- Bookings whose auto-archive deadline has passed (all groups) - cancelled and released
-- by the hourly archive job.
SELECT b.*
FROM bookings b
JOIN group_settings gs ON gs.group_id = b.group_id
WHERE (
    (b.status = 'draft' AND gs.draft_archive_days > 0
        AND b.created_at + (gs.draft_archive_days || ' days')::interval <= now())
    OR
    (b.status = 'rejected' AND gs.rejected_archive_days > 0
        AND b.updated_at + (gs.rejected_archive_days || ' days')::interval <= now())
);
