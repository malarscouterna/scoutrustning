-- +goose Up

CREATE TABLE article_events (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    group_id text NOT NULL REFERENCES groups(id),
    article_id uuid NOT NULL REFERENCES articles(id),
    actor_id text NOT NULL REFERENCES users(id),
    event_type text NOT NULL,
    description text NOT NULL DEFAULT '',
    metadata jsonb NOT NULL DEFAULT '{}',
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT article_events_type_check CHECK (event_type IN (
        'status_change', 'issue_reported', 'issue_resolved',
        'booked', 'picked_up', 'returned', 'note'
    ))
);

CREATE INDEX idx_article_events_article ON article_events(article_id);
CREATE INDEX idx_article_events_group ON article_events(group_id);

-- +goose Down

DROP TABLE IF EXISTS article_events;
