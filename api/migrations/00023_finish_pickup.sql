-- +goose Up

-- A leader can finish a partial pickup: items that were not actually collected
-- are marked 'not_picked_up' instead of being left reserved forever. Booking
-- status stays picked_up - the label shown in the UI is derived from item
-- progress - so already-collected items still go through the normal return
-- flow. Reservation/return/swap queries exclude this new status.
ALTER TABLE booking_items DROP CONSTRAINT booking_items_pickup_check;
ALTER TABLE booking_items ADD CONSTRAINT booking_items_pickup_check CHECK (pickup_status IS NULL OR pickup_status IN (
    'picked_up', 'swapped', 'not_picked_up'
));

-- Keep every type from migration 00022 and add 'pickup_finished'.
ALTER TABLE booking_events DROP CONSTRAINT booking_events_type_check;
ALTER TABLE booking_events ADD CONSTRAINT booking_events_type_check CHECK (event_type IN (
    'submitted', 'approved', 'rejected', 'cancelled', 'note',
    'items_changed', 'dates_changed', 'details_changed', 'auto_archived', 'swap',
    'reopened', 'pickup_finished'
));

-- +goose Down
DELETE FROM booking_events WHERE event_type = 'pickup_finished';
UPDATE booking_items SET pickup_status = NULL WHERE pickup_status = 'not_picked_up';

ALTER TABLE booking_events DROP CONSTRAINT booking_events_type_check;
ALTER TABLE booking_events ADD CONSTRAINT booking_events_type_check CHECK (event_type IN (
    'submitted', 'approved', 'rejected', 'cancelled', 'note',
    'items_changed', 'dates_changed', 'details_changed', 'auto_archived', 'swap',
    'reopened'
));
ALTER TABLE booking_items DROP CONSTRAINT booking_items_pickup_check;
ALTER TABLE booking_items ADD CONSTRAINT booking_items_pickup_check CHECK (pickup_status IS NULL OR pickup_status IN (
    'picked_up', 'swapped'
));
