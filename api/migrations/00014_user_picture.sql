-- +goose Up

-- Profile picture URL sourced from the Keycloak OIDC `picture` claim.
ALTER TABLE users ADD COLUMN IF NOT EXISTS picture text;

-- +goose Down
ALTER TABLE users DROP COLUMN IF EXISTS picture;
