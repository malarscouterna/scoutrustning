-- +goose Up

-- Issue reports (first-class issue entities)
CREATE TABLE issue_reports (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    group_id    text NOT NULL REFERENCES groups(id),
    title       text NOT NULL,
    description text NOT NULL,
    severity    text NOT NULL CHECK (severity IN ('usable', 'unusable', 'missing')),
    status      text NOT NULL DEFAULT 'open'
                  CHECK (status IN ('open', 'in_progress', 'resolved', 'archived')),
    reporter_id text NOT NULL REFERENCES users(id),
    booking_id  uuid REFERENCES bookings(id),
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);

-- Issue <-> article links
CREATE TABLE issue_articles (
    issue_id    uuid NOT NULL REFERENCES issue_reports(id) ON DELETE CASCADE,
    article_id  uuid NOT NULL REFERENCES articles(id),
    group_id    text NOT NULL REFERENCES groups(id),
    PRIMARY KEY (issue_id, article_id)
);

-- Issue assignees
CREATE TABLE issue_assignees (
    issue_id    uuid NOT NULL REFERENCES issue_reports(id) ON DELETE CASCADE,
    user_id     text NOT NULL REFERENCES users(id),
    group_id    text NOT NULL REFERENCES groups(id),
    assigned_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (issue_id, user_id)
);

-- Issue activity log
CREATE TABLE issue_events (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    issue_id    uuid NOT NULL REFERENCES issue_reports(id) ON DELETE CASCADE,
    group_id    text NOT NULL REFERENCES groups(id),
    actor_id    text NOT NULL REFERENCES users(id),
    event_type  text NOT NULL
                  CHECK (event_type IN ('comment', 'status_change', 'assignment', 'article_added', 'article_removed')),
    description text NOT NULL DEFAULT '',
    metadata    jsonb NOT NULL DEFAULT '{}',
    created_at  timestamptz NOT NULL DEFAULT now()
);

-- Indexes
CREATE INDEX idx_issue_reports_group ON issue_reports(group_id);
CREATE INDEX idx_issue_reports_status ON issue_reports(group_id, status);
CREATE INDEX idx_issue_reports_reporter ON issue_reports(reporter_id);
CREATE INDEX idx_issue_articles_issue ON issue_articles(issue_id);
CREATE INDEX idx_issue_articles_article ON issue_articles(article_id);
CREATE INDEX idx_issue_assignees_issue ON issue_assignees(issue_id);
CREATE INDEX idx_issue_assignees_user ON issue_assignees(user_id);
CREATE INDEX idx_issue_events_issue ON issue_events(issue_id);

-- Update articles.status: remove 'lost', add 'reported_missing'
ALTER TABLE articles DROP CONSTRAINT articles_status_check;
ALTER TABLE articles ADD CONSTRAINT articles_status_check CHECK (status IN (
    'ok', 'reported_usable', 'reported_unusable', 'reported_missing',
    'incoming', 'under_repair', 'archived'
));

-- Update booking_items return and pickup statuses: remove 'lost', add 'missing'
ALTER TABLE booking_items DROP CONSTRAINT booking_items_return_check;
ALTER TABLE booking_items ADD CONSTRAINT booking_items_return_check CHECK (return_status IS NULL OR return_status IN (
    'returned_ok', 'delayed', 'reported_usable', 'reported_unusable', 'missing', 'pending'
));

ALTER TABLE booking_items DROP CONSTRAINT booking_items_pickup_check;
ALTER TABLE booking_items ADD CONSTRAINT booking_items_pickup_check CHECK (pickup_status IS NULL OR pickup_status IN (
    'picked_up', 'swapped'
));

-- +goose Down
DROP TABLE IF EXISTS issue_events;
DROP TABLE IF EXISTS issue_assignees;
DROP TABLE IF EXISTS issue_articles;
DROP TABLE IF EXISTS issue_reports;

ALTER TABLE articles DROP CONSTRAINT articles_status_check;
ALTER TABLE articles ADD CONSTRAINT articles_status_check CHECK (status IN (
    'ok', 'reported_usable', 'incoming',
    'reported_unusable', 'under_repair', 'lost', 'archived'
));

ALTER TABLE booking_items DROP CONSTRAINT booking_items_return_check;
ALTER TABLE booking_items ADD CONSTRAINT booking_items_return_check CHECK (return_status IS NULL OR return_status IN (
    'returned_ok', 'delayed', 'reported_usable', 'reported_unusable', 'lost', 'pending'
));

ALTER TABLE booking_items DROP CONSTRAINT booking_items_pickup_check;
ALTER TABLE booking_items ADD CONSTRAINT booking_items_pickup_check CHECK (pickup_status IS NULL OR pickup_status IN (
    'picked_up', 'swapped', 'lost'
));
