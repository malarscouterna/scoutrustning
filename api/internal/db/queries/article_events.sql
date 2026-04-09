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

-- name: ListArticleEventsByGroup :many
-- Returns events for all articles matching a commercial_name + location, newest first.
SELECT ae.*,
    u.name AS actor_name,
    a.common_name AS article_name
FROM article_events ae
JOIN users u ON ae.actor_id = u.id
JOIN articles a ON ae.article_id = a.id
WHERE ae.group_id = @group_id
    AND ae.article_id IN (
        SELECT a2.id FROM articles a2
        WHERE a2.group_id = @group_id
            AND a2.commercial_name = @commercial_name
            AND a2.location_id = @location_id
    )
ORDER BY ae.created_at DESC;

-- name: ListArticleEventsByGroupLimited :many
SELECT ae.*,
    u.name AS actor_name,
    a.common_name AS article_name
FROM article_events ae
JOIN users u ON ae.actor_id = u.id
JOIN articles a ON ae.article_id = a.id
WHERE ae.group_id = @group_id
    AND ae.article_id IN (
        SELECT a2.id FROM articles a2
        WHERE a2.group_id = @group_id
            AND a2.commercial_name = @commercial_name
            AND a2.location_id = @location_id
    )
ORDER BY ae.created_at DESC
LIMIT @max_results;
