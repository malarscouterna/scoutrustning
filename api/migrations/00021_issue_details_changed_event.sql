-- +goose Up

-- Editing an issue report's title, description or severity (PUT /issues/{id}) logs a
-- 'details_changed' event with the old and new values in metadata.changes.
ALTER TABLE issue_events DROP CONSTRAINT issue_events_event_type_check;
ALTER TABLE issue_events ADD CONSTRAINT issue_events_event_type_check CHECK (event_type IN (
    'comment', 'status_change', 'assignment', 'article_added', 'article_removed', 'details_changed'
));

-- +goose Down
DELETE FROM issue_events WHERE event_type = 'details_changed';
ALTER TABLE issue_events DROP CONSTRAINT issue_events_event_type_check;
ALTER TABLE issue_events ADD CONSTRAINT issue_events_event_type_check CHECK (event_type IN (
    'comment', 'status_change', 'assignment', 'article_added', 'article_removed'
));
