-- +goose Up

DROP INDEX IF EXISTS idx_issue_reports_status;
DROP INDEX IF EXISTS idx_issue_reports_article;
DROP INDEX IF EXISTS idx_issue_reports_group;
DROP TABLE IF EXISTS issue_reports;

-- +goose Down

CREATE TABLE issue_reports (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    group_id text NOT NULL REFERENCES groups(id),
    article_id uuid NOT NULL REFERENCES articles(id),
    reporter_id text NOT NULL REFERENCES users(id),
    description text NOT NULL,
    severity text NOT NULL,
    status text NOT NULL DEFAULT 'open',
    resolution text,
    resolved_by text REFERENCES users(id),
    created_at timestamptz NOT NULL DEFAULT now(),
    resolved_at timestamptz,
    CONSTRAINT issue_reports_severity_check CHECK (severity IN ('usable', 'unusable')),
    CONSTRAINT issue_reports_status_check CHECK (status IN ('open', 'resolved'))
);

CREATE INDEX idx_issue_reports_group ON issue_reports(group_id);
CREATE INDEX idx_issue_reports_article ON issue_reports(article_id);
CREATE INDEX idx_issue_reports_status ON issue_reports(group_id, status);
