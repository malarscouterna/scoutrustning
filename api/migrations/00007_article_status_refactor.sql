-- +goose Up

-- Add new column
ALTER TABLE articles ADD COLUMN expected_available_date date;

-- Migrate drying_until values
UPDATE articles SET expected_available_date = drying_until WHERE drying_until IS NOT NULL;

-- Migrate statuses: booking-state statuses → ok, drying → ok, new → incoming
UPDATE articles SET status = 'ok' WHERE status IN ('booked', 'loaned', 'drying');
UPDATE articles SET status = 'incoming' WHERE status = 'new';

-- Drop old column
ALTER TABLE articles DROP COLUMN drying_until;

-- Replace status constraint
ALTER TABLE articles DROP CONSTRAINT articles_status_check;
ALTER TABLE articles ADD CONSTRAINT articles_status_check CHECK (status IN (
    'ok', 'reported_usable', 'incoming',
    'reported_unusable', 'under_repair', 'lost', 'archived'
));

-- +goose Down

ALTER TABLE articles ADD COLUMN drying_until date;
UPDATE articles SET drying_until = expected_available_date WHERE expected_available_date IS NOT NULL;

UPDATE articles SET status = 'new' WHERE status = 'incoming';

ALTER TABLE articles DROP COLUMN expected_available_date;

ALTER TABLE articles DROP CONSTRAINT articles_status_check;
ALTER TABLE articles ADD CONSTRAINT articles_status_check CHECK (status IN (
    'ok', 'reported_usable', 'reported_unusable',
    'under_repair', 'loaned', 'drying', 'booked',
    'archived', 'lost', 'new'
));
