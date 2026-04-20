-- +goose Up

-- Groups (multi-tenancy root, ID from Keycloak org)
CREATE TABLE groups (
    id text PRIMARY KEY,
    name text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

-- Users (ID from Keycloak member ID)
CREATE TABLE users (
    id text PRIMARY KEY,
    group_id text NOT NULL REFERENCES groups(id),
    name text NOT NULL,
    email text NOT NULL,
    notification_channel text NOT NULL DEFAULT 'email',
    gchat_webhook_url text,
    active_group_id text REFERENCES groups(id),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

-- Teams (troops, roles — populated from OIDC claims or by managers)
CREATE TABLE teams (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    group_id text NOT NULL REFERENCES groups(id),
    name text NOT NULL,
    type text NOT NULL DEFAULT 'troop',
    access_level text NOT NULL DEFAULT 'book',
    gchat_webhook_url text,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT teams_type_check CHECK (type IN ('troop', 'role')),
    CONSTRAINT teams_access_level_check CHECK (access_level IN ('view', 'book', 'trusted', 'manager')),
    UNIQUE (group_id, name, type)
);

-- Team claim mappings (OIDC claim → team)
CREATE TABLE team_claim_mappings (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    group_id text NOT NULL REFERENCES groups(id),
    team_id uuid NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    claim_scope text NOT NULL,
    claim_id text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT tcm_scope_check CHECK (claim_scope IN ('group', 'troop')),
    CONSTRAINT tcm_unique_claim UNIQUE (group_id, claim_scope, claim_id)
);

-- Locations
CREATE TABLE locations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    group_id text NOT NULL REFERENCES groups(id),
    name text NOT NULL,
    sort_order int NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now()
);

-- Categories (self-referencing for subcategories)
CREATE TABLE categories (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    group_id text NOT NULL REFERENCES groups(id),
    name text NOT NULL,
    parent_id uuid REFERENCES categories(id),
    sort_order int NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now()
);

-- Articles
CREATE TABLE articles (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    group_id text NOT NULL REFERENCES groups(id),
    commercial_name text NOT NULL DEFAULT '',
    common_name text NOT NULL,
    category_id uuid NOT NULL REFERENCES categories(id),
    location_id uuid NOT NULL REFERENCES locations(id),
    status text NOT NULL DEFAULT 'ok',
    individually_tracked boolean NOT NULL DEFAULT true,
    approval_level text NOT NULL DEFAULT 'none',
    image_ids jsonb NOT NULL DEFAULT '[]',
    description text NOT NULL DEFAULT '',
    instructions text NOT NULL DEFAULT '',
    manager_notes text NOT NULL DEFAULT '',
    purchase_date date,
    purchase_price numeric,
    place text NOT NULL DEFAULT '',
    expected_available_date date,
    import_batch_id uuid,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT articles_status_check CHECK (status IN (
        'ok', 'reported_usable', 'incoming',
        'reported_unusable', 'under_repair', 'lost', 'archived'
    )),
    CONSTRAINT articles_approval_level_check CHECK (approval_level IN ('none', 'low', 'high'))
);

-- Product images
CREATE TABLE product_images (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    file_id uuid NOT NULL,
    group_id text NOT NULL REFERENCES groups(id),
    uploaded_by text NOT NULL REFERENCES users(id),
    title text NOT NULL DEFAULT '',
    description text NOT NULL DEFAULT '',
    attribution text NOT NULL DEFAULT '',
    format text NOT NULL DEFAULT 'landscape',
    shared boolean NOT NULL DEFAULT false,
    is_reference boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT product_images_format_check CHECK (format IN ('landscape', 'portrait', 'square'))
);

-- Packages
CREATE TABLE packages (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    group_id text NOT NULL REFERENCES groups(id),
    name text NOT NULL,
    description text NOT NULL DEFAULT '',
    scope text NOT NULL DEFAULT 'org',
    owner_id text REFERENCES users(id),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT packages_scope_check CHECK (scope IN ('org', 'personal'))
);

-- Package items
CREATE TABLE package_items (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    group_id text NOT NULL REFERENCES groups(id),
    package_id uuid NOT NULL REFERENCES packages(id) ON DELETE CASCADE,
    category_id uuid REFERENCES categories(id),
    article_id uuid REFERENCES articles(id),
    quantity int NOT NULL DEFAULT 1
);

-- Bookings
CREATE TABLE bookings (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    group_id text NOT NULL REFERENCES groups(id),
    created_by text NOT NULL REFERENCES users(id),
    used_by_team_id uuid REFERENCES teams(id),
    used_by_external text,
    used_by_external_contact text,
    status text NOT NULL DEFAULT 'draft',
    start_date date NOT NULL,
    end_date date NOT NULL,
    notes text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT bookings_status_check CHECK (status IN (
        'draft', 'submitted', 'approved', 'rejected',
        'confirmed', 'picked_up', 'returned', 'cancelled'
    )),
    CONSTRAINT bookings_dates_check CHECK (end_date >= start_date)
);

-- Booking items
CREATE TABLE booking_items (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    group_id text NOT NULL REFERENCES groups(id),
    booking_id uuid NOT NULL REFERENCES bookings(id) ON DELETE CASCADE,
    article_id uuid NOT NULL REFERENCES articles(id),
    pickup_status text,
    return_status text,
    notes text NOT NULL DEFAULT '',
    CONSTRAINT booking_items_pickup_check CHECK (pickup_status IS NULL OR pickup_status IN (
        'picked_up', 'swapped', 'lost'
    )),
    CONSTRAINT booking_items_return_check CHECK (return_status IS NULL OR return_status IN (
        'returned_ok', 'delayed', 'reported_usable', 'reported_unusable', 'lost', 'pending'
    )),
    CONSTRAINT booking_items_unique_article UNIQUE (booking_id, article_id)
);

-- Article events
CREATE TABLE article_events (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    group_id text NOT NULL REFERENCES groups(id),
    article_id uuid NOT NULL REFERENCES articles(id),
    actor_id text NOT NULL REFERENCES users(id),
    event_type text NOT NULL,
    description text NOT NULL DEFAULT '',
    metadata jsonb NOT NULL DEFAULT '{}',
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT article_events_type_check CHECK (event_type IN (
        'status_change', 'issue_reported', 'issue_resolved',
        'booked', 'picked_up', 'returned', 'note', 'count_changed'
    ))
);

-- Booking events
CREATE TABLE booking_events (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    group_id text NOT NULL REFERENCES groups(id),
    booking_id uuid NOT NULL REFERENCES bookings(id) ON DELETE CASCADE,
    actor_id text NOT NULL REFERENCES users(id),
    event_type text NOT NULL,
    message text NOT NULL DEFAULT '',
    metadata jsonb NOT NULL DEFAULT '{}',
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT booking_events_type_check CHECK (event_type IN (
        'submitted', 'approved', 'rejected', 'cancelled', 'note',
        'items_changed', 'dates_changed', 'details_changed'
    ))
);

-- Group settings
CREATE TABLE group_settings (
    group_id text PRIMARY KEY REFERENCES groups(id),
    notification_email_from text NOT NULL DEFAULT '',
    smtp_key_encrypted bytea,
    gchat_webhook_url text NOT NULL DEFAULT '',
    default_approval_level text NOT NULL DEFAULT 'none',
    default_access_unknown text NOT NULL DEFAULT 'view',
    default_access_troop text NOT NULL DEFAULT 'book',
    default_access_role text NOT NULL DEFAULT 'book',
    image_upload_role text NOT NULL DEFAULT 'book',
    booking_role text NOT NULL DEFAULT 'book',
    article_edit_role text NOT NULL DEFAULT 'manager',
    issue_resolve_role text NOT NULL DEFAULT 'manager',
    manager_notes_role text NOT NULL DEFAULT 'manager',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT group_settings_approval_check CHECK (default_approval_level IN ('none', 'low', 'high')),
    CONSTRAINT gs_access_unknown_check CHECK (default_access_unknown IN ('view', 'book', 'trusted', 'manager')),
    CONSTRAINT gs_access_troop_check CHECK (default_access_troop IN ('view', 'book', 'trusted', 'manager')),
    CONSTRAINT gs_access_role_check CHECK (default_access_role IN ('view', 'book', 'trusted', 'manager')),
    CONSTRAINT gs_image_upload_role_check CHECK (image_upload_role IN ('view', 'book', 'trusted', 'manager')),
    CONSTRAINT gs_booking_role_check CHECK (booking_role IN ('book', 'trusted', 'manager')),
    CONSTRAINT gs_article_edit_role_check CHECK (article_edit_role IN ('book', 'trusted', 'manager')),
    CONSTRAINT gs_issue_resolve_role_check CHECK (issue_resolve_role IN ('book', 'trusted', 'manager')),
    CONSTRAINT gs_manager_notes_role_check CHECK (manager_notes_role IN ('trusted', 'manager'))
);

-- Audit log
CREATE TABLE audit_log (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    group_id text NOT NULL REFERENCES groups(id),
    user_id text NOT NULL REFERENCES users(id),
    action text NOT NULL,
    entity_type text NOT NULL,
    entity_id uuid NOT NULL,
    details jsonb NOT NULL DEFAULT '{}',
    created_at timestamptz NOT NULL DEFAULT now()
);

-- Indexes
CREATE INDEX idx_users_group ON users(group_id);
CREATE INDEX idx_teams_group ON teams(group_id);
CREATE INDEX idx_tcm_group ON team_claim_mappings(group_id);
CREATE INDEX idx_locations_group ON locations(group_id);
CREATE INDEX idx_categories_group ON categories(group_id);
CREATE INDEX idx_articles_group ON articles(group_id);
CREATE INDEX idx_articles_category ON articles(category_id);
CREATE INDEX idx_articles_location ON articles(location_id);
CREATE INDEX idx_articles_status ON articles(group_id, status);
CREATE INDEX idx_articles_import_batch ON articles(import_batch_id) WHERE import_batch_id IS NOT NULL;
CREATE INDEX idx_product_images_group ON product_images(group_id);
CREATE INDEX idx_product_images_shared ON product_images(shared) WHERE shared = true;
CREATE INDEX idx_packages_group ON packages(group_id);
CREATE INDEX idx_bookings_group ON bookings(group_id);
CREATE INDEX idx_bookings_dates ON bookings(group_id, start_date, end_date);
CREATE INDEX idx_bookings_status ON bookings(group_id, status);
CREATE INDEX idx_bookings_created_by ON bookings(created_by);
CREATE INDEX idx_bookings_team ON bookings(used_by_team_id);
CREATE INDEX idx_booking_items_booking ON booking_items(booking_id);
CREATE INDEX idx_booking_items_article ON booking_items(article_id);
CREATE INDEX idx_article_events_article ON article_events(article_id);
CREATE INDEX idx_article_events_group ON article_events(group_id);
CREATE INDEX idx_booking_events_booking ON booking_events(booking_id);
CREATE INDEX idx_booking_events_group ON booking_events(group_id);
CREATE INDEX idx_audit_log_group ON audit_log(group_id);
CREATE INDEX idx_audit_log_entity ON audit_log(entity_type, entity_id);

-- +goose Down
DROP TABLE IF EXISTS audit_log;
DROP TABLE IF EXISTS group_settings;
DROP TABLE IF EXISTS booking_events;
DROP TABLE IF EXISTS article_events;
DROP TABLE IF EXISTS booking_items;
DROP TABLE IF EXISTS bookings;
DROP TABLE IF EXISTS package_items;
DROP TABLE IF EXISTS packages;
DROP TABLE IF EXISTS product_images;
DROP TABLE IF EXISTS articles;
DROP TABLE IF EXISTS categories;
DROP TABLE IF EXISTS locations;
DROP TABLE IF EXISTS team_claim_mappings;
DROP TABLE IF EXISTS teams;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS groups;
