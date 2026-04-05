-- name: CreateArticleEvent :one
INSERT INTO article_events (
    group_id, article_id, actor_id, event_type, description, metadata
) VALUES (
    @group_id, @article_id, @actor_id, @event_type, @description, @metadata
)
RETURNING *;

-- name: ListArticleEvents :many
SELECT ae.*,
    u.name AS actor_name
FROM article_events ae
JOIN users u ON ae.actor_id = u.id
WHERE ae.article_id = @article_id AND ae.group_id = @group_id
ORDER BY ae.created_at DESC;

-- name: ListArticleEventsLimited :many
SELECT ae.*,
    u.name AS actor_name
FROM article_events ae
JOIN users u ON ae.actor_id = u.id
WHERE ae.article_id = @article_id AND ae.group_id = @group_id
ORDER BY ae.created_at DESC
LIMIT @max_results;
