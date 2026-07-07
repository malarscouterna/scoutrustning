-- +goose Up

-- Allow one member (users.id, the Keycloak member ID) to have a profile row
-- per registered scout group, rather than a single global row. Existing rows
-- are unaffected: id is already globally unique today, so every row trivially
-- satisfies the new composite key.

-- Drop the FKs referencing users(id) first - they depend on users_pkey.
ALTER TABLE product_images DROP CONSTRAINT product_images_uploaded_by_fkey;
ALTER TABLE packages DROP CONSTRAINT packages_owner_id_fkey;
ALTER TABLE bookings DROP CONSTRAINT bookings_created_by_fkey;
ALTER TABLE booking_events DROP CONSTRAINT booking_events_actor_id_fkey;
ALTER TABLE article_events DROP CONSTRAINT article_events_actor_id_fkey;
ALTER TABLE audit_log DROP CONSTRAINT audit_log_user_id_fkey;
ALTER TABLE issue_reports DROP CONSTRAINT issue_reports_reporter_id_fkey;
ALTER TABLE issue_assignees DROP CONSTRAINT issue_assignees_user_id_fkey;
ALTER TABLE issue_events DROP CONSTRAINT issue_events_actor_id_fkey;

ALTER TABLE users DROP CONSTRAINT users_pkey;
ALTER TABLE users ADD CONSTRAINT users_pkey PRIMARY KEY (id, group_id);

-- Every table below already carries its own group_id (multi-tenancy rule).
-- Widen each FK to composite (col, group_id) -> users(id, group_id) so a
-- referencing row can only point at the user's profile for its own group.
ALTER TABLE product_images ADD CONSTRAINT product_images_uploaded_by_fkey
    FOREIGN KEY (uploaded_by, group_id) REFERENCES users(id, group_id);

ALTER TABLE packages ADD CONSTRAINT packages_owner_id_fkey
    FOREIGN KEY (owner_id, group_id) REFERENCES users(id, group_id);

ALTER TABLE bookings ADD CONSTRAINT bookings_created_by_fkey
    FOREIGN KEY (created_by, group_id) REFERENCES users(id, group_id);

ALTER TABLE booking_events ADD CONSTRAINT booking_events_actor_id_fkey
    FOREIGN KEY (actor_id, group_id) REFERENCES users(id, group_id);

ALTER TABLE article_events ADD CONSTRAINT article_events_actor_id_fkey
    FOREIGN KEY (actor_id, group_id) REFERENCES users(id, group_id);

ALTER TABLE audit_log ADD CONSTRAINT audit_log_user_id_fkey
    FOREIGN KEY (user_id, group_id) REFERENCES users(id, group_id);

ALTER TABLE issue_reports ADD CONSTRAINT issue_reports_reporter_id_fkey
    FOREIGN KEY (reporter_id, group_id) REFERENCES users(id, group_id);

ALTER TABLE issue_assignees ADD CONSTRAINT issue_assignees_user_id_fkey
    FOREIGN KEY (user_id, group_id) REFERENCES users(id, group_id);

ALTER TABLE issue_events ADD CONSTRAINT issue_events_actor_id_fkey
    FOREIGN KEY (actor_id, group_id) REFERENCES users(id, group_id);

-- Active-group selection for multi-group members is resolved via a client-side
-- cookie (mirroring the existing dev-persona pattern), not stored server-side -
-- this column was added in 00001_init.sql but never wired up, drop it.
ALTER TABLE users DROP COLUMN active_group_id;

-- +goose Down

ALTER TABLE users ADD COLUMN active_group_id text REFERENCES groups(id);

ALTER TABLE issue_events DROP CONSTRAINT issue_events_actor_id_fkey;
ALTER TABLE issue_assignees DROP CONSTRAINT issue_assignees_user_id_fkey;
ALTER TABLE issue_reports DROP CONSTRAINT issue_reports_reporter_id_fkey;
ALTER TABLE audit_log DROP CONSTRAINT audit_log_user_id_fkey;
ALTER TABLE article_events DROP CONSTRAINT article_events_actor_id_fkey;
ALTER TABLE booking_events DROP CONSTRAINT booking_events_actor_id_fkey;
ALTER TABLE bookings DROP CONSTRAINT bookings_created_by_fkey;
ALTER TABLE packages DROP CONSTRAINT packages_owner_id_fkey;
ALTER TABLE product_images DROP CONSTRAINT product_images_uploaded_by_fkey;

ALTER TABLE users DROP CONSTRAINT users_pkey;
ALTER TABLE users ADD CONSTRAINT users_pkey PRIMARY KEY (id);

ALTER TABLE issue_events ADD CONSTRAINT issue_events_actor_id_fkey
    FOREIGN KEY (actor_id) REFERENCES users(id);

ALTER TABLE issue_assignees ADD CONSTRAINT issue_assignees_user_id_fkey
    FOREIGN KEY (user_id) REFERENCES users(id);

ALTER TABLE issue_reports ADD CONSTRAINT issue_reports_reporter_id_fkey
    FOREIGN KEY (reporter_id) REFERENCES users(id);

ALTER TABLE audit_log ADD CONSTRAINT audit_log_user_id_fkey
    FOREIGN KEY (user_id) REFERENCES users(id);

ALTER TABLE article_events ADD CONSTRAINT article_events_actor_id_fkey
    FOREIGN KEY (actor_id) REFERENCES users(id);

ALTER TABLE booking_events ADD CONSTRAINT booking_events_actor_id_fkey
    FOREIGN KEY (actor_id) REFERENCES users(id);

ALTER TABLE bookings ADD CONSTRAINT bookings_created_by_fkey
    FOREIGN KEY (created_by) REFERENCES users(id);

ALTER TABLE packages ADD CONSTRAINT packages_owner_id_fkey
    FOREIGN KEY (owner_id) REFERENCES users(id);

ALTER TABLE product_images ADD CONSTRAINT product_images_uploaded_by_fkey
    FOREIGN KEY (uploaded_by) REFERENCES users(id);
