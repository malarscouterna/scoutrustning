-- +goose Up

-- Booking events: conversation history for approval flow and general notes
CREATE TABLE booking_events (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    group_id text NOT NULL REFERENCES groups(id),
    booking_id uuid NOT NULL REFERENCES bookings(id) ON DELETE CASCADE,
    actor_id text NOT NULL REFERENCES users(id),
    event_type text NOT NULL,
    message text NOT NULL DEFAULT '',
    metadata jsonb NOT NULL DEFAULT '{}',
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT booking_events_type_check CHECK (event_type IN (
        'submitted', 'approved', 'rejected', 'cancelled', 'note',
        'items_changed', 'dates_changed', 'details_changed'
    ))
);

CREATE INDEX idx_booking_events_booking ON booking_events(booking_id);
CREATE INDEX idx_booking_events_group ON booking_events(group_id);

-- Drop single-shot approval fields (replaced by booking_events)
ALTER TABLE bookings DROP COLUMN IF EXISTS approval_message;
ALTER TABLE bookings DROP COLUMN IF EXISTS approved_by;
ALTER TABLE bookings DROP COLUMN IF EXISTS approved_at;

-- +goose Down

ALTER TABLE bookings ADD COLUMN approval_message text NOT NULL DEFAULT '';
ALTER TABLE bookings ADD COLUMN approved_by text REFERENCES users(id);
ALTER TABLE bookings ADD COLUMN approved_at timestamptz;

DROP TABLE IF EXISTS booking_events;
