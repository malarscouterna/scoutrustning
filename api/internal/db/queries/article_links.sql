-- name: ListArticleLinksForScope :many
SELECT * FROM article_links
WHERE group_id = @group_id AND scope_key = @scope_key
ORDER BY created_at, id;

-- name: ListArticleLinksForScopes :many
-- Visible links for article/booking/availability responses. Trusted links are
-- only included for viewers whose access level allows it.
SELECT * FROM article_links
WHERE group_id = @group_id
    AND scope_key = ANY(@scope_keys::text[])
    AND (visibility = 'public' OR sqlc.arg(include_trusted)::boolean)
ORDER BY created_at, id;

-- name: UpsertArticleLink :one
-- Editing a link keeps its checker state: only label/visibility change.
INSERT INTO article_links (group_id, scope_key, label, url, visibility, created_by)
VALUES (@group_id, @scope_key, @label, @url, @visibility, @created_by)
ON CONFLICT (group_id, scope_key, url) DO UPDATE SET
    label = excluded.label,
    visibility = excluded.visibility,
    updated_at = now()
RETURNING *;

-- name: DeleteArticleLink :execrows
DELETE FROM article_links
WHERE id = @id AND group_id = @group_id AND scope_key = @scope_key;

-- name: DeleteArticleLinksForScope :exec
DELETE FROM article_links WHERE group_id = @group_id AND scope_key = @scope_key;

-- name: DeleteArticleLinksNotIn :exec
DELETE FROM article_links
WHERE group_id = @group_id
    AND scope_key = @scope_key
    AND NOT (url = ANY(@urls::text[]));

-- name: MoveArticleLinks :execrows
UPDATE article_links SET scope_key = @new_scope_key, updated_at = now()
WHERE group_id = @group_id AND scope_key = @old_scope_key;

-- name: DeleteOrphanedArticleLinks :exec
DELETE FROM article_links al
WHERE al.group_id = @group_id
    AND al.scope_key = @scope_key
    AND NOT EXISTS (
        SELECT 1 FROM articles a
        WHERE a.group_id = al.group_id
            AND COALESCE(NULLIF(a.commercial_name, ''), a.common_name) = al.scope_key
    );

-- name: GetArticleScopeKey :one
SELECT COALESCE(NULLIF(commercial_name, ''), common_name)::text AS scope_key
FROM articles
WHERE id = @id AND group_id = @group_id;

-- name: ListArticleLinksDueForCheck :many
SELECT * FROM article_links
WHERE last_checked_at IS NULL OR last_checked_at < @checked_before
ORDER BY last_checked_at NULLS FIRST
LIMIT @max_count;

-- name: UpdateArticleLinkCheck :exec
UPDATE article_links SET
    last_checked_at = now(),
    last_status = @last_status,
    last_http_status = @last_http_status,
    last_error = @last_error,
    failure_kind = @failure_kind,
    acknowledged_kind = CASE WHEN @last_status = 'ok' THEN '' ELSE acknowledged_kind END,
    acknowledged_at = CASE WHEN @last_status = 'ok' THEN NULL ELSE acknowledged_at END,
    acknowledged_by = CASE WHEN @last_status = 'ok' THEN '' ELSE acknowledged_by END
WHERE id = @id;

-- name: AcknowledgeArticleLink :execrows
-- "Mark as working" for the current failure kind. If the link later fails in a
-- different way (e.g. 403 -> 404) the acknowledgement no longer applies.
UPDATE article_links SET
    acknowledged_kind = failure_kind,
    acknowledged_at = now(),
    acknowledged_by = @user_id
WHERE id = @id AND group_id = @group_id AND last_status = 'broken';

-- name: UnacknowledgeArticleLink :execrows
UPDATE article_links SET
    acknowledged_kind = '',
    acknowledged_at = NULL,
    acknowledged_by = ''
WHERE id = @id AND group_id = @group_id;

-- name: ListBrokenArticleLinks :many
-- All currently failing links, including acknowledged ones (flagged so the UI
-- can show them muted with an undo action).
SELECT al.*,
    (al.acknowledged_kind <> '' AND al.acknowledged_kind = al.failure_kind) AS acknowledged,
    COALESCE(
        (
            SELECT COALESCE(NULLIF(a.commercial_name, ''), a.common_name)
            FROM articles a
            WHERE a.group_id = al.group_id
                AND COALESCE(NULLIF(a.commercial_name, ''), a.common_name) = al.scope_key
            ORDER BY a.created_at
            LIMIT 1
        ),
        al.scope_key
    )::text AS article_name
FROM article_links al
WHERE al.group_id = @group_id
    AND al.last_status = 'broken'
ORDER BY acknowledged, al.last_checked_at DESC, al.label;

-- name: ArticleScopeInUse :one
-- Whether any article other than the given one still belongs to this product
-- type. Used when renaming, so links are only moved when the old type is gone.
SELECT EXISTS (
    SELECT 1 FROM articles
    WHERE group_id = @group_id
        AND COALESCE(NULLIF(commercial_name, ''), common_name) = @scope_key
        AND id != @exclude_id
) AS in_use;

-- name: CountArticleLinksForScope :one
SELECT COUNT(*) FROM article_links
WHERE group_id = @group_id AND scope_key = @scope_key;

-- name: ListArticleLinksForGroup :many
-- Every link in a group, used by the "check now" action in group settings.
-- Oldest check first so repeated runs eventually cover everything.
SELECT * FROM article_links
WHERE group_id = @group_id
ORDER BY last_checked_at NULLS FIRST, id
LIMIT @max_count;

-- name: CountArticleLinksForGroup :one
SELECT COUNT(*) FROM article_links
WHERE group_id = @group_id;

-- name: DeleteArticleLinkDuplicates :exec
-- Before moving links to a renamed product type, drop rows that would collide
-- with links the target scope already has (same url).
DELETE FROM article_links n
USING article_links o
WHERE n.group_id = @group_id
    AND n.scope_key = @new_scope_key
    AND o.group_id = @group_id
    AND o.scope_key = @old_scope_key
    AND n.url = o.url;
