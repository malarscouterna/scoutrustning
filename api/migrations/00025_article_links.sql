-- +goose Up

-- Links that describe an article/product type: user guides, repair manuals, etc.
-- Links are scoped to the product type rather than a single physical article so
-- every copy (same commercial_name, or common_name when there is no commercial
-- name) shares the same set - matching how description/instructions propagate.
-- Checker state lives on the row so the daily job can update it in place.
CREATE TABLE article_links (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    group_id text NOT NULL REFERENCES groups(id),
    scope_key text NOT NULL,
    label text NOT NULL,
    url text NOT NULL,
    visibility text NOT NULL DEFAULT 'public',
    created_by text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    last_checked_at timestamptz,
    last_status text NOT NULL DEFAULT 'unchecked',
    last_http_status integer,
    last_error text NOT NULL DEFAULT '',
    failure_kind text NOT NULL DEFAULT '',
    acknowledged_kind text NOT NULL DEFAULT '',
    acknowledged_at timestamptz,
    acknowledged_by text NOT NULL DEFAULT '',
    CONSTRAINT article_links_visibility_check CHECK (visibility IN ('public', 'trusted')),
    CONSTRAINT article_links_status_check CHECK (last_status IN ('unchecked', 'ok', 'broken')),
    CONSTRAINT article_links_url_check CHECK (url ~* '^https?://[^[:space:]]+$'),
    CONSTRAINT article_links_label_check CHECK (length(btrim(label)) BETWEEN 1 AND 120),
    CONSTRAINT article_links_created_by_fkey FOREIGN KEY (created_by, group_id) REFERENCES users(id, group_id)
);

CREATE UNIQUE INDEX article_links_group_scope_url_idx ON article_links (group_id, scope_key, url);
CREATE INDEX article_links_group_scope_idx ON article_links (group_id, scope_key);
CREATE INDEX article_links_check_idx ON article_links (last_checked_at NULLS FIRST);

-- +goose Down

DROP TABLE article_links;
