-- +goose Up

-- Editing a pre-checkout booking (details, add/remove items, or the explicit
-- reopen endpoint) returns it to draft, so the manager's approval queue no
-- longer holds a booking the leader is reworking. Logged as a 'reopened'
-- booking event with metadata.from = the status it reopened from. Rejected
-- bookings keep their existing behavior (no event: the rejection reason must
-- remain the last message in the history).
-- Keep every type from migration 00020 and add 'reopened'.
ALTER TABLE booking_events DROP CONSTRAINT booking_events_type_check;
ALTER TABLE booking_events ADD CONSTRAINT booking_events_type_check CHECK (event_type IN (
    'submitted', 'approved', 'rejected', 'cancelled', 'note',
    'items_changed', 'dates_changed', 'details_changed', 'auto_archived', 'swap',
    'reopened'
));

-- +goose Down
DELETE FROM booking_events WHERE event_type = 'reopened';
ALTER TABLE booking_events DROP CONSTRAINT booking_events_type_check;
ALTER TABLE booking_events ADD CONSTRAINT booking_events_type_check CHECK (event_type IN (
    'submitted', 'approved', 'rejected', 'cancelled', 'note',
    'items_changed', 'dates_changed', 'details_changed', 'auto_archived', 'swap'
));
