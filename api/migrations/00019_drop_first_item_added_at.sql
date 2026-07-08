-- +goose Up

-- Superseded: the draft auto-archive deadline now runs from bookings.created_at instead
-- of first-item-add, so the countdown (and the old separate, non-configurable 48h
-- empty-draft cleanup, which this replaces) applies from the moment a draft is created,
-- with or without items yet.
ALTER TABLE bookings DROP COLUMN IF EXISTS first_item_added_at;

-- +goose Down
ALTER TABLE bookings ADD COLUMN first_item_added_at timestamptz;
