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
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

-- Units (managed entity, populated from OIDC or by admins)
CREATE TABLE units (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    group_id text NOT NULL REFERENCES groups(id),
    name text NOT NULL,
    gchat_webhook_url text,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (group_id, name)
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
    requires_approval boolean NOT NULL DEFAULT false,
    image_path text,
    description text NOT NULL DEFAULT '',
    instructions text NOT NULL DEFAULT '',
    purchase_date date,
    purchase_price numeric,
    place text NOT NULL DEFAULT '',
    drying_until date,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT articles_status_check CHECK (status IN (
        'ok', 'reported_usable', 'reported_unusable',
        'under_repair', 'loaned', 'drying', 'booked',
        'archived', 'new'
    ))
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
    used_by_unit_id uuid REFERENCES units(id),
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
        'picked_up', 'swapped', 'not_available'
    )),
    CONSTRAINT booking_items_return_check CHECK (return_status IS NULL OR return_status IN (
        'returned_ok', 'delayed', 'broken', 'lost', 'pending'
    )),
    CONSTRAINT booking_items_unique_article UNIQUE (booking_id, article_id)
);

-- Issue reports
CREATE TABLE issue_reports (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    group_id text NOT NULL REFERENCES groups(id),
    article_id uuid NOT NULL REFERENCES articles(id),
    reporter_id text NOT NULL REFERENCES users(id),
    description text NOT NULL,
    severity text NOT NULL,
    status text NOT NULL DEFAULT 'open',
    resolution text,
    resolved_by text REFERENCES users(id),
    created_at timestamptz NOT NULL DEFAULT now(),
    resolved_at timestamptz,
    CONSTRAINT issue_reports_severity_check CHECK (severity IN ('usable', 'unusable')),
    CONSTRAINT issue_reports_status_check CHECK (status IN ('open', 'resolved'))
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
CREATE INDEX idx_units_group ON units(group_id);
CREATE INDEX idx_locations_group ON locations(group_id);
CREATE INDEX idx_categories_group ON categories(group_id);
CREATE INDEX idx_articles_group ON articles(group_id);
CREATE INDEX idx_articles_category ON articles(category_id);
CREATE INDEX idx_articles_location ON articles(location_id);
CREATE INDEX idx_articles_status ON articles(group_id, status);
CREATE INDEX idx_packages_group ON packages(group_id);
CREATE INDEX idx_bookings_group ON bookings(group_id);
CREATE INDEX idx_bookings_dates ON bookings(group_id, start_date, end_date);
CREATE INDEX idx_bookings_status ON bookings(group_id, status);
CREATE INDEX idx_bookings_created_by ON bookings(created_by);
CREATE INDEX idx_bookings_unit ON bookings(used_by_unit_id);
CREATE INDEX idx_booking_items_booking ON booking_items(booking_id);
CREATE INDEX idx_booking_items_article ON booking_items(article_id);
CREATE INDEX idx_issue_reports_group ON issue_reports(group_id);
CREATE INDEX idx_issue_reports_article ON issue_reports(article_id);
CREATE INDEX idx_issue_reports_status ON issue_reports(group_id, status);
CREATE INDEX idx_audit_log_group ON audit_log(group_id);
CREATE INDEX idx_audit_log_entity ON audit_log(entity_type, entity_id);

-- Seed data: Mälarscouterna
INSERT INTO groups (id, name) VALUES ('766', 'Mälarscouterna');

INSERT INTO locations (group_id, name, sort_order) VALUES
    ('766', 'Kammaren', 1),
    ('766', 'Östergården', 2),
    ('766', 'Ladan', 3),
    ('766', 'Kallförrådet', 4),
    ('766', 'Hajkförrådet', 5),
    ('766', 'Magasinet', 6),
    ('766', 'Verkstan', 7);

INSERT INTO categories (group_id, name, sort_order) VALUES
    ('766', 'Övrigt', 1);

-- +goose Down
DROP TABLE IF EXISTS audit_log;
DROP TABLE IF EXISTS issue_reports;
DROP TABLE IF EXISTS booking_items;
DROP TABLE IF EXISTS bookings;
DROP TABLE IF EXISTS package_items;
DROP TABLE IF EXISTS packages;
DROP TABLE IF EXISTS articles;
DROP TABLE IF EXISTS categories;
DROP TABLE IF EXISTS locations;
DROP TABLE IF EXISTS units;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS groups;
