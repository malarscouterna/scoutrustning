-- +goose Up

-- Per-group auto-archive timeouts (docs/implementation/pre-release.md "Booking auto-archive setting").
-- 0 disables auto-archiving for that stage. The existing 48h empty-draft cleanup
-- (CleanupEmptyDrafts) is unrelated and unaffected by these settings.
ALTER TABLE group_settings ADD COLUMN draft_archive_days integer NOT NULL DEFAULT 3;
ALTER TABLE group_settings ADD COLUMN rejected_archive_days integer NOT NULL DEFAULT 7;

-- Set once, the first time an item is added to a draft booking; never reset on later
-- edits or item removal, so the auto-archive deadline stays fixed from that moment.
ALTER TABLE bookings ADD COLUMN first_item_added_at timestamptz;

ALTER TABLE booking_events DROP CONSTRAINT booking_events_type_check;
ALTER TABLE booking_events ADD CONSTRAINT booking_events_type_check CHECK (event_type IN (
    'submitted', 'approved', 'rejected', 'cancelled', 'note',
    'items_changed', 'dates_changed', 'details_changed', 'auto_archived'
));

-- +goose Down
ALTER TABLE booking_events DROP CONSTRAINT booking_events_type_check;
ALTER TABLE booking_events ADD CONSTRAINT booking_events_type_check CHECK (event_type IN (
    'submitted', 'approved', 'rejected', 'cancelled', 'note',
    'items_changed', 'dates_changed', 'details_changed'
));
ALTER TABLE bookings DROP COLUMN IF EXISTS first_item_added_at;
ALTER TABLE group_settings DROP COLUMN IF EXISTS rejected_archive_days;
ALTER TABLE group_settings DROP COLUMN IF EXISTS draft_archive_days;
