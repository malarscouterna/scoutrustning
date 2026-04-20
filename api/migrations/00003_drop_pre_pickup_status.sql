-- +goose Up
ALTER TABLE bookings DROP COLUMN IF EXISTS pre_pickup_status;

-- +goose Down
ALTER TABLE bookings ADD COLUMN IF NOT EXISTS pre_pickup_status text;
