-- +goose Up
ALTER TABLE bookings ADD COLUMN pre_pickup_status text;

-- +goose Down
ALTER TABLE bookings DROP COLUMN pre_pickup_status;
