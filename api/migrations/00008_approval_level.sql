-- +goose Up

-- Convert requires_approval boolean to approval_level text enum
ALTER TABLE articles ADD COLUMN approval_level text NOT NULL DEFAULT 'none';
UPDATE articles SET approval_level = CASE WHEN requires_approval THEN 'low' ELSE 'none' END;
ALTER TABLE articles DROP COLUMN requires_approval;
ALTER TABLE articles ADD CONSTRAINT articles_approval_level_check CHECK (approval_level IN ('none', 'low', 'high'));

-- Add approval_message to bookings (shown to leader on approve/reject)
ALTER TABLE bookings ADD COLUMN approval_message text NOT NULL DEFAULT '';

-- +goose Down

ALTER TABLE bookings DROP COLUMN approval_message;

ALTER TABLE articles DROP CONSTRAINT articles_approval_level_check;
ALTER TABLE articles ADD COLUMN requires_approval boolean NOT NULL DEFAULT false;
UPDATE articles SET requires_approval = (approval_level != 'none');
ALTER TABLE articles DROP COLUMN approval_level;
