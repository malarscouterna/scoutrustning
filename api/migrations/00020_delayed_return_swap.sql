-- +goose Up

-- Persists the expected return date a manager enters when marking an item delayed
-- (docs/pre-release.md "Delayed return - conflict handling"). Previously required by
-- the API but never stored anywhere. Used for the conflict-overview warning and the
-- "next expected user" preview shown when marking an item delayed.
ALTER TABLE booking_items ADD COLUMN expected_return_date date;

ALTER TABLE booking_events DROP CONSTRAINT booking_events_type_check;
ALTER TABLE booking_events ADD CONSTRAINT booking_events_type_check CHECK (event_type IN (
    'submitted', 'approved', 'rejected', 'cancelled', 'note',
    'items_changed', 'dates_changed', 'details_changed', 'auto_archived', 'swap'
));

-- +goose Down
ALTER TABLE booking_events DROP CONSTRAINT booking_events_type_check;
ALTER TABLE booking_events ADD CONSTRAINT booking_events_type_check CHECK (event_type IN (
    'submitted', 'approved', 'rejected', 'cancelled', 'note',
    'items_changed', 'dates_changed', 'details_changed', 'auto_archived'
));
ALTER TABLE booking_items DROP COLUMN IF EXISTS expected_return_date;
