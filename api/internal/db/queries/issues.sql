-- name: CreateIssue :one
INSERT INTO issue_reports (group_id, title, description, severity, reporter_id, booking_id)
VALUES (@group_id, @title, @description, @severity, @reporter_id, sqlc.narg('booking_id')::uuid)
RETURNING *;

-- name: GetIssue :one
SELECT
    ir.*,
    u.name AS reporter_name
FROM issue_reports ir
JOIN users u ON ir.reporter_id = u.id
WHERE ir.id = @id AND ir.group_id = @group_id;

-- name: ListIssues :many
-- Filters: status (comma-separated via array), mine (reporter_id = user_id OR assigned),
-- article_id (linked articles).
SELECT
    ir.*,
    u.name AS reporter_name
FROM issue_reports ir
JOIN users u ON ir.reporter_id = u.id
WHERE ir.group_id = @group_id
    AND (COALESCE(array_length(@statuses::text[], 1), 0) = 0 OR ir.status = ANY(@statuses::text[]))
    AND (NOT @mine::bool OR (
        ir.reporter_id = @user_id
        OR ir.id IN (
            SELECT issue_id FROM issue_assignees WHERE user_id = @user_id AND group_id = @group_id
        )
        OR ir.id IN (
            SELECT issue_id FROM issue_events WHERE actor_id = @user_id AND group_id = @group_id
        )
    ))
    AND (sqlc.narg('article_id')::uuid IS NULL OR ir.id IN (
        SELECT issue_id FROM issue_articles WHERE article_id = sqlc.narg('article_id') AND group_id = @group_id
    ))
ORDER BY ir.updated_at DESC, ir.created_at DESC;

-- name: UpdateIssue :one
UPDATE issue_reports SET
    title       = COALESCE(sqlc.narg('title')::text, title),
    description = COALESCE(sqlc.narg('description')::text, description),
    status      = COALESCE(sqlc.narg('status')::text, status),
    updated_at  = now()
WHERE id = @id AND group_id = @group_id
RETURNING *;

-- name: CreateIssueEvent :one
INSERT INTO issue_events (issue_id, group_id, actor_id, event_type, description, metadata)
VALUES (@issue_id, @group_id, @actor_id, @event_type, @description, @metadata)
RETURNING *;

-- name: ListIssueEvents :many
SELECT
    ie.*,
    u.name AS actor_name
FROM issue_events ie
JOIN users u ON ie.actor_id = u.id
WHERE ie.issue_id = @issue_id AND ie.group_id = @group_id
ORDER BY ie.created_at ASC;

-- name: AddIssueArticle :exec
INSERT INTO issue_articles (issue_id, article_id, group_id)
VALUES (@issue_id, @article_id, @group_id)
ON CONFLICT DO NOTHING;

-- name: RemoveIssueArticle :exec
DELETE FROM issue_articles
WHERE issue_id = @issue_id AND article_id = @article_id AND group_id = @group_id;

-- name: ListIssueArticles :many
SELECT
    a.id,
    a.commercial_name,
    a.common_name,
    l.name AS location_name,
    a.individually_tracked
FROM issue_articles ia
JOIN articles a ON ia.article_id = a.id
JOIN locations l ON a.location_id = l.id
WHERE ia.issue_id = @issue_id AND ia.group_id = @group_id;

-- name: ListIssueAssignees :many
SELECT
    ia.user_id,
    ia.assigned_at,
    u.name AS user_name
FROM issue_assignees ia
JOIN users u ON ia.user_id = u.id
WHERE ia.issue_id = @issue_id AND ia.group_id = @group_id;

-- name: ReplaceIssueAssignees :exec
-- Caller should do delete + insert in a transaction; this deletes all first.
DELETE FROM issue_assignees WHERE issue_id = @issue_id AND group_id = @group_id;

-- name: AddIssueAssignee :exec
INSERT INTO issue_assignees (issue_id, user_id, group_id)
VALUES (@issue_id, @user_id, @group_id)
ON CONFLICT DO NOTHING;

-- name: DeriveArticleStatus :one
-- Re-derives article status from its open issues.
-- Returns the new status that should be stored.
-- Priority: missing = unusable > usable > ok (missing takes precedence for tiebreak).
SELECT CASE
    WHEN EXISTS (
        SELECT 1 FROM issue_articles ia
        JOIN issue_reports ir ON ia.issue_id = ir.id
        WHERE ia.article_id = @article_id AND ia.group_id = @group_id
          AND ir.status IN ('open', 'in_progress') AND ir.severity = 'missing'
    ) THEN 'reported_missing'
    WHEN EXISTS (
        SELECT 1 FROM issue_articles ia
        JOIN issue_reports ir ON ia.issue_id = ir.id
        WHERE ia.article_id = @article_id AND ia.group_id = @group_id
          AND ir.status IN ('open', 'in_progress') AND ir.severity = 'unusable'
    ) THEN 'reported_unusable'
    WHEN EXISTS (
        SELECT 1 FROM issue_articles ia
        JOIN issue_reports ir ON ia.issue_id = ir.id
        WHERE ia.article_id = @article_id AND ia.group_id = @group_id
          AND ir.status IN ('open', 'in_progress') AND ir.severity = 'usable'
    ) THEN 'reported_usable'
    ELSE 'ok'
END AS derived_status;

-- name: UpdateArticleStatusDirect :one
-- Direct status update used by issue derivation (no expected_available_date reset).
UPDATE articles SET status = @status, updated_at = now()
WHERE id = @id AND group_id = @group_id
RETURNING *;

-- name: ListOpenIssueArticles :many
-- Returns all article IDs linked to a specific issue that are currently open/in_progress.
SELECT ia.article_id
FROM issue_articles ia
WHERE ia.issue_id = @issue_id AND ia.group_id = @group_id;

-- name: ArchiveOpenIssuesForArticle :exec
-- When an article is archived, close all open/in_progress issues linked only to it.
-- Issues linked to other non-archived articles are left open.
UPDATE issue_reports ir SET status = 'archived', updated_at = now()
WHERE ir.group_id = @group_id
  AND ir.status IN ('open', 'in_progress')
  AND ir.id IN (
      SELECT ia1.issue_id FROM issue_articles ia1
      WHERE ia1.article_id = @article_id AND ia1.group_id = @group_id
  )
  AND ir.id NOT IN (
      SELECT ia2.issue_id FROM issue_articles ia2
      JOIN articles a ON ia2.article_id = a.id
      WHERE ia2.group_id = @group_id AND ia2.article_id != @article_id AND a.status != 'archived'
  );

-- name: GetArticleCurrentStatus :one
SELECT status FROM articles WHERE id = @id AND group_id = @group_id;
