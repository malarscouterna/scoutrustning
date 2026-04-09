-- +goose Up
ALTER TABLE article_events DROP CONSTRAINT article_events_type_check;
ALTER TABLE article_events ADD CONSTRAINT article_events_type_check CHECK (event_type IN (
    'status_change', 'issue_reported', 'issue_resolved',
    'booked', 'picked_up', 'returned', 'note', 'count_changed'
));

-- +goose Down
ALTER TABLE article_events DROP CONSTRAINT article_events_type_check;
ALTER TABLE article_events ADD CONSTRAINT article_events_type_check CHECK (event_type IN (
    'status_change', 'issue_reported', 'issue_resolved',
    'booked', 'picked_up', 'returned', 'note'
));
