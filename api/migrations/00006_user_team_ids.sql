-- +goose Up

-- Denormalized list of team UUIDs a user belongs to, updated on every login.
-- Enables querying "all members of team X" for notification recipient resolution
-- without rejoining through OIDC claim mappings.
ALTER TABLE users ADD COLUMN team_ids uuid[] NOT NULL DEFAULT '{}';

-- +goose Down
ALTER TABLE users DROP COLUMN IF EXISTS team_ids;
