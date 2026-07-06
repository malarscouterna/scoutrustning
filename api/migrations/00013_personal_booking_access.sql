-- +goose Up

-- Group-level access switch controlling who can create personal bookings
-- (bookings with no team, e.g. a leader borrowing for a personal or
-- external activity). Follows the same view/book/trusted/manager pattern
-- as the other per-group access switches.
ALTER TABLE group_settings ADD COLUMN IF NOT EXISTS personal_booking_role text NOT NULL DEFAULT 'book';
ALTER TABLE group_settings ADD CONSTRAINT gs_personal_booking_role_check
    CHECK (personal_booking_role IN ('view', 'book', 'trusted', 'manager'));

-- +goose Down
ALTER TABLE group_settings DROP CONSTRAINT IF EXISTS gs_personal_booking_role_check;
ALTER TABLE group_settings DROP COLUMN IF EXISTS personal_booking_role;
