-- +goose Up

-- Add 'lost' to article status
ALTER TABLE articles DROP CONSTRAINT articles_status_check;
ALTER TABLE articles ADD CONSTRAINT articles_status_check CHECK (status IN (
    'ok', 'reported_usable', 'reported_unusable',
    'under_repair', 'loaned', 'drying', 'booked',
    'archived', 'lost', 'new'
));

-- Migrate existing data before changing constraint
UPDATE booking_items SET return_status = 'reported_unusable' WHERE return_status = 'broken';

-- Update return_status constraint
ALTER TABLE booking_items DROP CONSTRAINT booking_items_return_check;
ALTER TABLE booking_items ADD CONSTRAINT booking_items_return_check CHECK (return_status IS NULL OR return_status IN (
    'returned_ok', 'delayed', 'reported_usable', 'reported_unusable', 'lost', 'pending'
));

-- Replace not_available with lost in pickup_status
UPDATE booking_items SET pickup_status = 'lost' WHERE pickup_status = 'not_available';
ALTER TABLE booking_items DROP CONSTRAINT booking_items_pickup_check;
ALTER TABLE booking_items ADD CONSTRAINT booking_items_pickup_check CHECK (pickup_status IS NULL OR pickup_status IN (
    'picked_up', 'swapped', 'lost'
));

-- +goose Down

ALTER TABLE booking_items DROP CONSTRAINT booking_items_return_check;
UPDATE booking_items SET return_status = 'broken' WHERE return_status = 'reported_unusable';
ALTER TABLE booking_items ADD CONSTRAINT booking_items_return_check CHECK (return_status IS NULL OR return_status IN (
    'returned_ok', 'delayed', 'broken', 'lost', 'pending'
));

ALTER TABLE booking_items DROP CONSTRAINT booking_items_pickup_check;
UPDATE booking_items SET pickup_status = 'not_available' WHERE pickup_status = 'lost';
ALTER TABLE booking_items ADD CONSTRAINT booking_items_pickup_check CHECK (pickup_status IS NULL OR pickup_status IN (
    'picked_up', 'swapped', 'not_available'
));

ALTER TABLE articles DROP CONSTRAINT articles_status_check;
ALTER TABLE articles ADD CONSTRAINT articles_status_check CHECK (status IN (
    'ok', 'reported_usable', 'reported_unusable',
    'under_repair', 'loaned', 'drying', 'booked',
    'archived', 'new'
));
