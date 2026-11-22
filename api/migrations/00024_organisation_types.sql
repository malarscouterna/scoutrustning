-- +goose Up

-- Scoutrustning supports more than just scout groups. A tenant (groups row)
-- now records which Scoutnet entity type it represents and the external ID in
-- that entity's namespace. Districts, regions, organisations, corps, networks
-- and projects live in separate ID namespaces from groups, so their internal
-- tenant key is prefixed. Existing group tenants keep their raw Scoutnet group
-- number as their id, so no child-table data needs to change.

ALTER TABLE groups ADD COLUMN org_type text NOT NULL DEFAULT 'group';
ALTER TABLE groups ADD COLUMN scoutnet_id text;
UPDATE groups SET scoutnet_id = id;
ALTER TABLE groups ALTER COLUMN scoutnet_id SET NOT NULL;

ALTER TABLE groups ADD CONSTRAINT groups_org_type_check CHECK (org_type IN (
    'group', 'district', 'region', 'organisation', 'corps', 'network', 'project'
));

ALTER TABLE groups ADD CONSTRAINT groups_scoutnet_unique UNIQUE (org_type, scoutnet_id);

-- The internal key is the raw Scoutnet group number for groups (backwards
-- compatible), and "<type>:<scoutnet_id>" for every other entity type.
ALTER TABLE groups ADD CONSTRAINT groups_tenant_key_check CHECK (
    (org_type = 'group' AND id = scoutnet_id)
    OR (org_type <> 'group' AND id = org_type || ':' || scoutnet_id)
);

-- +goose Down

ALTER TABLE groups DROP CONSTRAINT groups_tenant_key_check;
ALTER TABLE groups DROP CONSTRAINT groups_scoutnet_unique;
ALTER TABLE groups DROP CONSTRAINT groups_org_type_check;
ALTER TABLE groups DROP COLUMN scoutnet_id;
ALTER TABLE groups DROP COLUMN org_type;
