-- name: InsertProductImage :one
INSERT INTO product_images (id, file_id, group_id, uploaded_by, title, description, format, shared, attribution, is_reference)
VALUES (@id, @file_id, @group_id, @uploaded_by, @title, @description, @format, @shared, @attribution, @is_reference)
RETURNING *;

-- name: GetProductImage :one
SELECT pi.*, u.name AS uploaded_by_name, g.name AS uploaded_by_group
FROM product_images pi
JOIN users u ON pi.uploaded_by = u.id
JOIN groups g ON pi.group_id = g.id
WHERE pi.id = @id;

-- name: ListProductImagesByIds :many
-- Returns images matching the given file_ids, in order.
SELECT pi.*, u.name AS uploaded_by_name, g.name AS uploaded_by_group
FROM product_images pi
JOIN users u ON pi.uploaded_by = u.id
JOIN groups g ON pi.group_id = g.id
WHERE pi.file_id = ANY(@ids::uuid[])
ORDER BY array_position(@ids::uuid[], pi.file_id);

-- name: ListSharedImages :many
-- Returns original uploads (not references) that are shared or from the current group.
SELECT pi.*, u.name AS uploaded_by_name, g.name AS uploaded_by_group
FROM product_images pi
JOIN users u ON pi.uploaded_by = u.id
JOIN groups g ON pi.group_id = g.id
WHERE pi.is_reference = false
    AND (pi.shared = true OR pi.group_id = @group_id)
    AND (sqlc.narg('search')::text IS NULL
        OR pi.title ILIKE '%' || sqlc.narg('search') || '%'
        OR pi.description ILIKE '%' || sqlc.narg('search') || '%')
ORDER BY pi.created_at DESC;

-- name: DeleteProductImage :exec
DELETE FROM product_images WHERE id = @id AND group_id = @group_id;

-- name: UpdateProductImage :one
UPDATE product_images
SET title = @title, description = @description, shared = @shared, attribution = @attribution
WHERE id = @id AND group_id = @group_id
RETURNING *;

-- name: CountProductImagesByFileId :one
-- Counts how many product_images rows reference the same file on disk.
SELECT COUNT(*) FROM product_images WHERE file_id = @file_id;

-- name: ListProductImagesByUploader :many
-- Returns original uploads (not references) by a specific user with usage counts.
SELECT pi.*, u.name AS uploaded_by_name, g.name AS uploaded_by_group,
    (SELECT COUNT(*) FROM product_images p2 WHERE p2.file_id = pi.file_id AND p2.group_id = pi.group_id) AS own_group_count,
    (SELECT COUNT(*) FROM product_images p3 WHERE p3.file_id = pi.file_id AND p3.group_id != pi.group_id) AS other_group_count
FROM product_images pi
JOIN users u ON pi.uploaded_by = u.id
JOIN groups g ON pi.group_id = g.id
WHERE pi.uploaded_by = @user_id AND pi.group_id = @group_id AND pi.is_reference = false
ORDER BY pi.created_at DESC;

-- name: ListArticlesUsingImage :many
-- Returns articles that reference a given image ID in their image_ids array.
SELECT a.id, a.commercial_name, a.common_name, a.individually_tracked,
    l.name AS location_name
FROM articles a
JOIN locations l ON a.location_id = l.id
WHERE a.group_id = @group_id
    AND a.image_ids @> jsonb_build_array(@image_id_str::text)
ORDER BY a.commercial_name, a.common_name;

-- name: GetImageUploadRole :one
SELECT image_upload_role FROM group_settings WHERE group_id = @group_id;

-- name: RemoveImageIdFromAllArticles :execrows
-- Removes a specific image ID from the image_ids array on all articles in a group.
UPDATE articles
SET image_ids = image_ids - @image_id_str, updated_at = now()
WHERE group_id = @group_id
    AND image_ids @> jsonb_build_array(@image_id_str);
