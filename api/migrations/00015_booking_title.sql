-- +goose Up
ALTER TABLE bookings RENAME COLUMN notes TO title;
ALTER TABLE booking_items DROP COLUMN notes;

-- +goose Down
ALTER TABLE booking_items ADD COLUMN notes text NOT NULL DEFAULT '';
ALTER TABLE bookings RENAME COLUMN title TO notes;
