-- +goose Up

-- Drop all FK constraints referencing users(id) — will be re-added as composite below.
-- IF EXISTS guards against environments where a constraint was created under a non-default name.
ALTER TABLE product_images   DROP CONSTRAINT IF EXISTS product_images_uploaded_by_fkey;
ALTER TABLE packages         DROP CONSTRAINT IF EXISTS packages_owner_id_fkey;
ALTER TABLE bookings         DROP CONSTRAINT IF EXISTS bookings_created_by_fkey;
ALTER TABLE article_events   DROP CONSTRAINT IF EXISTS article_events_actor_id_fkey;
ALTER TABLE booking_events   DROP CONSTRAINT IF EXISTS booking_events_actor_id_fkey;
ALTER TABLE audit_log        DROP CONSTRAINT IF EXISTS audit_log_user_id_fkey;
ALTER TABLE issue_reports    DROP CONSTRAINT IF EXISTS issue_reports_reporter_id_fkey;
ALTER TABLE issue_assignees  DROP CONSTRAINT IF EXISTS issue_assignees_user_id_fkey;
ALTER TABLE issue_events     DROP CONSTRAINT IF EXISTS issue_events_actor_id_fkey;
ALTER TABLE notification_log DROP CONSTRAINT IF EXISTS notification_log_user_id_fkey;

-- Change users PK to composite (id, group_id).
-- A user may belong to multiple groups with separate settings per group.
ALTER TABLE users DROP CONSTRAINT users_pkey;
ALTER TABLE users ADD PRIMARY KEY (id, group_id);
ALTER TABLE users DROP COLUMN active_group_id;

-- Re-add FK constraints as composite to enforce group-scoped integrity.
ALTER TABLE product_images   ADD CONSTRAINT product_images_uploaded_by_fkey
    FOREIGN KEY (uploaded_by, group_id) REFERENCES users(id, group_id);
ALTER TABLE packages         ADD CONSTRAINT packages_owner_id_fkey
    FOREIGN KEY (owner_id, group_id)   REFERENCES users(id, group_id);
ALTER TABLE bookings         ADD CONSTRAINT bookings_created_by_fkey
    FOREIGN KEY (created_by, group_id) REFERENCES users(id, group_id);
ALTER TABLE article_events   ADD CONSTRAINT article_events_actor_id_fkey
    FOREIGN KEY (actor_id, group_id)   REFERENCES users(id, group_id);
ALTER TABLE booking_events   ADD CONSTRAINT booking_events_actor_id_fkey
    FOREIGN KEY (actor_id, group_id)   REFERENCES users(id, group_id);
ALTER TABLE audit_log        ADD CONSTRAINT audit_log_user_id_fkey
    FOREIGN KEY (user_id, group_id)    REFERENCES users(id, group_id);
ALTER TABLE issue_reports    ADD CONSTRAINT issue_reports_reporter_id_fkey
    FOREIGN KEY (reporter_id, group_id) REFERENCES users(id, group_id);
ALTER TABLE issue_assignees  ADD CONSTRAINT issue_assignees_user_id_fkey
    FOREIGN KEY (user_id, group_id)    REFERENCES users(id, group_id);
ALTER TABLE issue_events     ADD CONSTRAINT issue_events_actor_id_fkey
    FOREIGN KEY (actor_id, group_id)   REFERENCES users(id, group_id);
ALTER TABLE notification_log ADD CONSTRAINT notification_log_user_id_fkey
    FOREIGN KEY (user_id, group_id)    REFERENCES users(id, group_id);

-- +goose Down

ALTER TABLE product_images   DROP CONSTRAINT product_images_uploaded_by_fkey;
ALTER TABLE packages         DROP CONSTRAINT packages_owner_id_fkey;
ALTER TABLE bookings         DROP CONSTRAINT bookings_created_by_fkey;
ALTER TABLE article_events   DROP CONSTRAINT article_events_actor_id_fkey;
ALTER TABLE booking_events   DROP CONSTRAINT booking_events_actor_id_fkey;
ALTER TABLE audit_log        DROP CONSTRAINT audit_log_user_id_fkey;
ALTER TABLE issue_reports    DROP CONSTRAINT issue_reports_reporter_id_fkey;
ALTER TABLE issue_assignees  DROP CONSTRAINT issue_assignees_user_id_fkey;
ALTER TABLE issue_events     DROP CONSTRAINT issue_events_actor_id_fkey;
ALTER TABLE notification_log DROP CONSTRAINT notification_log_user_id_fkey;

ALTER TABLE users ADD COLUMN active_group_id text REFERENCES groups(id);
ALTER TABLE users DROP CONSTRAINT users_pkey;
ALTER TABLE users ADD PRIMARY KEY (id);

ALTER TABLE product_images   ADD CONSTRAINT product_images_uploaded_by_fkey
    FOREIGN KEY (uploaded_by) REFERENCES users(id);
ALTER TABLE packages         ADD CONSTRAINT packages_owner_id_fkey
    FOREIGN KEY (owner_id)    REFERENCES users(id);
ALTER TABLE bookings         ADD CONSTRAINT bookings_created_by_fkey
    FOREIGN KEY (created_by)  REFERENCES users(id);
ALTER TABLE article_events   ADD CONSTRAINT article_events_actor_id_fkey
    FOREIGN KEY (actor_id)    REFERENCES users(id);
ALTER TABLE booking_events   ADD CONSTRAINT booking_events_actor_id_fkey
    FOREIGN KEY (actor_id)    REFERENCES users(id);
ALTER TABLE audit_log        ADD CONSTRAINT audit_log_user_id_fkey
    FOREIGN KEY (user_id)     REFERENCES users(id);
ALTER TABLE issue_reports    ADD CONSTRAINT issue_reports_reporter_id_fkey
    FOREIGN KEY (reporter_id) REFERENCES users(id);
ALTER TABLE issue_assignees  ADD CONSTRAINT issue_assignees_user_id_fkey
    FOREIGN KEY (user_id)     REFERENCES users(id);
ALTER TABLE issue_events     ADD CONSTRAINT issue_events_actor_id_fkey
    FOREIGN KEY (actor_id)    REFERENCES users(id);
ALTER TABLE notification_log ADD CONSTRAINT notification_log_user_id_fkey
    FOREIGN KEY (user_id)     REFERENCES users(id);
