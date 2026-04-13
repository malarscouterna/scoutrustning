-- +goose Up

ALTER TABLE units ADD COLUMN type text NOT NULL DEFAULT 'unit';
ALTER TABLE units ADD CONSTRAINT units_type_check CHECK (type IN ('unit', 'project'));

-- Update unique constraint to include type (allows "Valborg" as both unit and project)
ALTER TABLE units DROP CONSTRAINT units_group_id_name_key;
ALTER TABLE units ADD CONSTRAINT units_group_id_name_type_key UNIQUE (group_id, name, type);

-- +goose Down

ALTER TABLE units DROP CONSTRAINT units_group_id_name_type_key;
ALTER TABLE units ADD CONSTRAINT units_group_id_name_key UNIQUE (group_id, name);
ALTER TABLE units DROP CONSTRAINT units_type_check;
ALTER TABLE units DROP COLUMN type;
