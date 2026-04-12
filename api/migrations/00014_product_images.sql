-- +goose Up

CREATE TABLE product_images (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    file_id uuid NOT NULL,
    group_id text NOT NULL REFERENCES groups(id),
    uploaded_by text NOT NULL REFERENCES users(id),
    title text NOT NULL DEFAULT '',
    description text NOT NULL DEFAULT '',
    format text NOT NULL DEFAULT 'landscape',
    shared boolean NOT NULL DEFAULT false,
    name_display text NOT NULL DEFAULT 'first_name',
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT product_images_format_check CHECK (format IN ('landscape', 'portrait', 'square')),
    CONSTRAINT product_images_name_display_check CHECK (name_display IN ('first_name', 'full_name'))
);

CREATE INDEX idx_product_images_group ON product_images(group_id);
CREATE INDEX idx_product_images_shared ON product_images(shared) WHERE shared = true;

-- Migrate existing image_ids UUIDs into product_images rows.
-- Each UUID in the jsonb array becomes a row with id = file_id (same UUID for both).
-- We pick the first user in the group as uploaded_by.
INSERT INTO product_images (id, file_id, group_id, uploaded_by)
SELECT DISTINCT (elem.value #>> '{}')::uuid, (elem.value #>> '{}')::uuid, a.group_id,
    (SELECT u.id FROM users u WHERE u.group_id = a.group_id ORDER BY u.created_at LIMIT 1)
FROM articles a, jsonb_array_elements(a.image_ids) AS elem(value)
WHERE jsonb_array_length(a.image_ids) > 0;

ALTER TABLE group_settings ADD COLUMN image_upload_role text NOT NULL DEFAULT 'leader';
ALTER TABLE group_settings ADD CONSTRAINT group_settings_image_upload_role_check
    CHECK (image_upload_role IN ('any', 'leader', 'project_leader', 'equipment_manager'));

-- +goose Down

ALTER TABLE group_settings DROP CONSTRAINT IF EXISTS group_settings_image_upload_role_check;
ALTER TABLE group_settings DROP COLUMN IF EXISTS image_upload_role;

DROP TABLE IF EXISTS product_images;
