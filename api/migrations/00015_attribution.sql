-- +goose Up

ALTER TABLE product_images ADD COLUMN attribution text NOT NULL DEFAULT '';
ALTER TABLE product_images DROP CONSTRAINT product_images_name_display_check;
ALTER TABLE product_images DROP COLUMN name_display;

-- +goose Down

ALTER TABLE product_images ADD COLUMN name_display text NOT NULL DEFAULT 'first_name';
ALTER TABLE product_images ADD CONSTRAINT product_images_name_display_check
    CHECK (name_display IN ('first_name', 'full_name'));
ALTER TABLE product_images DROP COLUMN attribution;
