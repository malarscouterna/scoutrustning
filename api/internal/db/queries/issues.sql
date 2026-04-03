-- name: CreateIssueReport :one
INSERT INTO issue_reports (
    group_id, article_id, reporter_id, description, severity, status
) VALUES (
    @group_id, @article_id, @reporter_id, @description, @severity, 'open'
)
RETURNING *;

-- name: ListIssueReports :many
SELECT ir.*,
    a.commercial_name,
    a.common_name,
    u.name AS reporter_name
FROM issue_reports ir
JOIN articles a ON ir.article_id = a.id
JOIN users u ON ir.reporter_id = u.id
WHERE ir.group_id = @group_id
    AND (sqlc.narg('status')::text IS NULL OR ir.status = sqlc.narg('status'))
ORDER BY ir.created_at DESC;

-- name: GetIssueReport :one
SELECT ir.*,
    a.commercial_name,
    a.common_name,
    u.name AS reporter_name
FROM issue_reports ir
JOIN articles a ON ir.article_id = a.id
JOIN users u ON ir.reporter_id = u.id
WHERE ir.id = @id AND ir.group_id = @group_id;
