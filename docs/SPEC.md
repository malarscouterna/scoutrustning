# ms-utrustning — Equipment Booking Service

An equipment booking service for Mälarscouterna (and eventually other scout groups). Leaders book scouting equipment (tents, stoves, knives, etc.), pick it up with a checklist, and return it. Equipment managers maintain inventory and handle issue reports.

## Context

This is a standalone service, unrelated to the ScoutSync project in ms-integrations. Different maintainers, different users.

## Roles

All roles come from OIDC token claims (ScoutID via Keycloak). Users are identified by their Keycloak member ID (e.g. `3000924`), groups by their Keycloak org ID (e.g. `766` for Mälarscouterna).

- **Leader** — can book from bookable articles. Some articles require equipment manager approval. Can report issues on any article.
- **Project leader** — can book anything without approval. Can report issues.
- **Equipment manager** — full inventory CRUD, manages issue reports, approves bookings that require it, moves articles between locations/statuses.

All authenticated users can view equipment and report issues.

## Core Concepts

### Articles

An article is either:
- **Individually tracked** — a specific item with its own identity (e.g. "Sibley 10", "Primus 12"). Bookings reference the specific item.
- **Quantity tracked** — a type of item where we track count, not individuals (e.g. "liggunderlag", "hajkbricka"). Bookings reserve a quantity.

Both types are modeled as individual article records in the database. For quantity-tracked items, each record represents one unit, but the UI groups them and users book by count.

**Identifying tracking type**: Individually tracked items have distinct names (e.g. "Sibley 1", "Sibley 2"). Quantity-tracked items share the same name and don't need individual identification at pickup — the checklist just says "grab 5 Tältlampa LED" instead of listing specific items.

**Setting up quantity-tracked items**: The CSV import creates one record per row. For items that should be quantity-tracked (e.g. LED tent lights where you have 47 but the spreadsheet only has 1 row), the equipment manager marks the item as quantity-tracked in the UI and sets the actual count, which creates the additional records. This keeps import simple and puts inventory knowledge with the manager.

**Article assignment**: When booking, users don't choose specific items — they book "3 tents" or "10 liggunderlag". The system assigns specific articles when the booking is confirmed (based on availability and location). At pickup, the checklist shows which specific items to collect (e.g. "Sibley 10 — shelf 3 in Hajkförrådet"). During pickup, the user can override assignments: swap one assigned article for another available one if needed.

Article fields:
- Commercial name (product type for grouping and booking, e.g. "Sibley", "Stormkök"). Users browse and book by this name — "I need 3 Sibley tents". This is what distinguishes a Sibley from a Primus.
- Common name (individual item identifier, e.g. "Sibley 1", "Sibley 2"). Used for physical identification at pickup — a label/sticker on the item. The numbering is just a convention; the database tracks by UUID.
- Category / subcategory (broad grouping for filtering, e.g. Sova, Mat, Säkerhet, Verktyg)
- Location (where it physically is)
- Status (determines availability)
- Image (stored locally)
- Description
- Usage instructions
- Purchase date and price
- Place (free text — where within the location, e.g. "shelf 3")
- Whether it's individually tracked or quantity-tracked
- Whether it requires approval to book (for leaders)

### Locations

Per-group, admin-configured. Mälarscouterna's initial set:
Kammaren, Östergården, Ladan, Kallförrådet, Hajkförrådet, Magasinet, Verkstan (under repair)

### Statuses

Statuses determine whether an article is bookable. Per-group configurable in the future, but start with a fixed set:

| Status | Bookable? |
|---|---|
| OK | Yes |
| Reported — usable | Yes |
| Reported — unusable | No |
| Under repair | No |
| Loaned | No |
| Drying | No |
| Booked | No |
| Archived | No |
| New (not in circulation) | No |

### Packages

Predefined sets of articles for common scenarios (e.g. "2-dagars hajk för 8 utmanare", "brända mandlar-kittet"). A package is a template — when a user selects one, it populates their booking cart which they can then adjust.

Scoped to:
- **Org-wide** — visible to everyone, managed by equipment managers
- **Personal** — saved by individual users for their own reuse

Unit/group-level scoping deferred to later.

### Bookings

A booking is a reservation of articles for a date range (day granularity), created by a person.

**Booking ownership**: A booking has a creator (the logged-in user) and a "used by" field which can be:
- A unit (e.g. "Yggdrasil") — all leaders of that unit can see the booking, do pickup, do (partial) returns, and manage it. Units are a managed entity (database table), populated from OIDC claims or created by equipment managers. Unit membership comes from OIDC group claims. Leaders can only book for units they belong to.
- A project (e.g. "Valborg 2026") — same as units but for temporary cross-unit activities. Project leaders can only book for projects they belong to. Projects bypass article approval requirements. Both units and projects are stored in the `units` table with a `type` column (`unit` or `project`).
- An external person (free-text name + contact info) — only the creator and equipment managers can manage it.
- Empty — personal booking, only the creator manages it.
- Equipment managers can book for any unit, project, or external person.

This means partial returns by different leaders are natural: leader A picks up on Friday, leader B returns half on Sunday, leader C returns the rest on Monday. All see the same booking because they share the unit.

#### Booking lifecycle

```
Draft → Submitted → [Approved] → Confirmed → Picked up → Returned
                  → Rejected
```

- **Draft** — user is building their cart
- **Submitted** — booking requested. If no articles in the booking have `requires_approval = true`, auto-transitions to Confirmed. If any article requires approval, the whole booking waits.
- **Approved/Rejected** — equipment manager acts on bookings that need approval. Approval auto-transitions to Confirmed.
- **Confirmed** — booking is locked in, articles reserved
- **Picked up** — user has collected the equipment. Per-article checklist shows which specific items to collect and where to find them. Tick off each item. Can swap assigned items for other available ones and add extras during pickup.
- **Returned** — user has returned equipment. Per-article checklist on return. Each article can be marked with a return status:
  - Returned OK
  - Delayed (item not returned yet, expected return date set, booking stays open)
  - Broken (triggers issue report)
  - Lost (triggers issue report, article status updated)

#### Availability

An article is available for a date range if:
- Its status is bookable (OK or Reported — usable)
- It is not assigned to an overlapping confirmed/picked-up/submitted/approved booking where it hasn't been fully returned (return_status is NULL or 'delayed')

Delayed items keep the booking open and the article unavailable. The booker remains responsible until the item is returned. Drying is handled as a separate article inventory status (set by the manager after return), not as a return status.

### Issue Reports

Any authenticated user can report an issue on any article at any time (including while it's loaned out). Reporting sets the article status to "Reported — usable" or "Reported — unusable" and creates an `issue_reported` article event with the description.

There is no separate issue table. An article with a reported status *is* an open issue. Equipment managers resolve issues by changing the article status back to OK (or to under repair, archived, etc.) via the Ärenden page. Every status change is logged as an article event with an optional comment, forming the full issue history.

See `docs/issues-and-events.md` for design details.

### Users

Users authenticate via ScoutID (OIDC/Keycloak). The OIDC token contains name, email, group memberships, and roles.

We do need a lightweight user table to store:
- Keycloak member ID — primary key (text, e.g. `"3000924"`)
- Cached name/email (from token, updated on login)
- Notification preferences (email vs Google Chat)
- Personal packages

Users are upserted on login from token claims. No registration flow.

### Notifications

Used for:
- Approval requests → equipment managers
- Booking confirmations → borrower
- Overdue reminders → borrower + equipment managers
- Issue report notifications → equipment managers

Channels:
- Email
- Google Chat (messages to shared spaces)

Google Chat notifications are configured per scout unit — each unit has a Chat space where booking notifications are sent. Admin notifications (approvals, issue reports) go to a dedicated admin Chat space with all equipment managers. Spaces are configured as webhook URLs per group.

Per-user preference for which channel. Start with whichever is easiest to implement (likely email), add the other in v1.1.

### Multi-tenancy

Every table has a `group_id` column. Every query filters on it. The group is derived from the OIDC token's group/org claims.

For v1, there is one group (Mälarscouterna). The data model supports multiple groups from day one so we don't have to retrofit it.

What's deferred:
- Group onboarding UI
- Per-group OIDC configuration
- Cross-group anything

## Tech Stack

| Component | Choice | Rationale |
|---|---|---|
| Backend | Go (Chi + pgx + sqlc) | API-first, single binary, minimal dependencies, long-term stable |
| Frontend | SvelteKit | Lightweight, SSR + client hydration, responsive, good mobile feel |
| UI components | @scouterna/ui-webc (Stencil web components) | Scouting design system, works natively in SvelteKit as custom elements |
| Design tokens | @scouterna/tailwind-theme + @scouterna/design-tokens | Consistent styling |
| Database | PostgreSQL | Relational model, date range queries for availability |
| DB queries | sqlc | Type-safe Go from SQL, no ORM magic |
| DB migrations | goose v3 | SQL file based, single-file up/down format |
| Auth | Auth.js (@auth/sveltekit) with Keycloak provider, JWT verification in Go | SvelteKit handles OIDC flow, Go API validates tokens |
| Image storage | Local Docker volume | Simple, sufficient for scale |
| Deployment | Docker Compose (Go API + SvelteKit + Postgres) | Behind existing reverse proxy on VPS |

### Architecture

```
┌───────────┐     ┌────────────────────┐     ┌──────────────────┐
│ ScoutID   │     │    SvelteKit       │     │    Go API        │
│ Keycloak  │◄───►│                    │────►│    (Chi)         │
│  (OIDC)   │     │  - @scouterna/     │     │                  │
└───────────┘     │    ui-webc         │     │  - /api/v0/*     │
                  │  - responsive      │     │  - JWT verify    │
                  │  - mobile-first    │     │  - group_id      │
                  └────────────────────┘     │    scoping       │
                                             │            │     │
                                             │  ┌─────────┴──┐  │
                                             │  │ PostgreSQL  │  │
                                             │  └────────────┘  │
                                             │  ┌────────────┐  │
                                             │  │ images/    │  │
                                             │  │ (volume)   │  │
                                             │  └────────────┘  │
                                             └──────────────────┘
```

Reverse proxy routes:
- `/api/*` → Go API container
- `/*` → SvelteKit container

### OIDC flow

1. User visits SvelteKit app
2. SvelteKit server-side checks for session; if none, redirects to ScoutID Keycloak
3. Keycloak authenticates, redirects back with code
4. SvelteKit exchanges code for tokens, stores in httpOnly cookie
5. On API calls, SvelteKit passes the access token to Go API
6. Go API validates JWT using Keycloak's JWKS endpoint, extracts claims (member_id, name, email, roles, units, projects, group_id)
7. Go API upserts user record (member_id + cached profile) and scopes all queries to the user's group

## User Flows

### Leader: Book equipment

1. Browse articles by category, search by name, or select a package. Filter and sort by category and location.
2. See availability for desired dates
3. Add to cart, adjust quantities
4. Set "used by" — a unit (e.g. "Yggdrasil"), external person, or leave empty for personal
5. Submit booking
6. If approval needed → wait for equipment manager
7. Receive confirmation (email or Google Chat)

### Leader: Pick up equipment

Any leader with access to the booking (creator, or unit member if unit booking) can do pickup.

1. Open confirmed booking
2. See checklist of reserved articles, filterable and sortable by category and location
3. See which specific items are assigned and where to find them
4. Tick off each item as picked up
5. If an item is unavailable or wrong → swap for another available one
6. Can add extra items
7. Confirm pickup

### Leader: Return equipment

Any leader with access to the booking can do (partial) returns. Different leaders can return different items at different times.

1. Open active (picked up) booking
2. Per-article return checklist, filterable and sortable by category and location
3. For each article, set return status: OK / Delayed / Broken / Lost
4. Broken/Lost auto-creates issue report
5. Delayed sets an expected return date, booking stays open
6. Confirm return (partial returns keep booking open)

### Equipment manager: Inventory

1. Dashboard: article counts per location, per status
2. CRUD articles (single + bulk)
3. Move articles between locations
4. Change article status
5. Manage categories and locations
6. Create/edit org-wide packages

### Equipment manager: Issues

1. Queue of open issue reports
2. View details, update status, add resolution notes
3. Mark resolved (updates article status)

### Equipment manager: Approvals

1. Queue of bookings awaiting approval
2. View booking details
3. Approve or reject with optional message

## Data Model (key tables)

All tables have `group_id` (text, FK → groups). Omitted below for brevity.

### groups
| Column | Type | Notes |
|---|---|---|
| id | text | PK, Keycloak org ID (e.g. "766") |
| name | text | e.g. "Mälarscouterna" |
| created_at | timestamptz | |

### users
| Column | Type | Notes |
|---|---|---|
| id | text | PK, Keycloak member ID (e.g. "3000924") |
| name | text | From token, updated on login |
| email | text | From token |
| notification_channel | text | 'email' or 'gchat' |
| gchat_webhook_url | text | Nullable |
| created_at / updated_at | timestamptz | |

### locations
| Column | Type | Notes |
|---|---|---|
| id | uuid | PK |
| name | text | |
| sort_order | int | |

### categories
| Column | Type | Notes |
|---|---|---|
| id | uuid | PK |
| name | text | e.g. "Tält" |
| parent_id | uuid | Nullable, FK → categories (subcategories) |
| sort_order | int | |

### articles
| Column | Type | Notes |
|---|---|---|
| id | uuid | PK |
| commercial_name | text | Product type (e.g. "Sibley") — what users browse and book by |
| common_name | text | Individual item name (e.g. "Sibley 1") — for physical identification |
| category_id | uuid | FK → categories |
| location_id | uuid | FK → locations |
| status | text | Enum: ok, reported_usable, reported_unusable, under_repair, loaned, drying, booked, archived, new |
| individually_tracked | boolean | |
| requires_approval | boolean | |
| image_path | text | Nullable |
| description | text | |
| instructions | text | |
| purchase_date | date | Nullable |
| purchase_price | numeric | Nullable |
| place | text | Free text, where within location |
| drying_until | date | Nullable, set on return |
| created_at / updated_at | timestamptz | |

### packages
| Column | Type | Notes |
|---|---|---|
| id | uuid | PK |
| name | text | |
| description | text | |
| scope | text | 'org' or 'personal' |
| owner_id | text | Nullable, FK → users (for personal) |
| created_at / updated_at | timestamptz | |

### package_items
| Column | Type | Notes |
|---|---|---|
| id | uuid | PK |
| package_id | uuid | FK → packages |
| category_id | uuid | FK → categories (for quantity items: "give me 10 from this category") |
| article_id | uuid | Nullable, FK → articles (for specific items) |
| quantity | int | Default 1 |

### units
| Column | Type | Notes |
|---|---|---|
| id | uuid | PK |
| name | text | e.g. "Yggdrasil", unique per group + type |
| type | text | `unit` or `project`, default `unit` |
| gchat_webhook_url | text | Nullable, for unit notifications |
| created_at | timestamptz | |

### bookings
| Column | Type | Notes |
|---|---|---|
| id | uuid | PK |
| created_by | text | FK → users |
| used_by_unit_id | uuid | Nullable, FK → units |
| used_by_external | text | Nullable, free text for external borrowers |
| used_by_external_contact | text | Nullable |
| status | text | draft, submitted, approved, rejected, confirmed, picked_up, returned, cancelled |
| start_date | date | |
| end_date | date | |
| notes | text | |
| created_at / updated_at | timestamptz | |

### booking_items
| Column | Type | Notes |
|---|---|---|
| id | uuid | PK |
| booking_id | uuid | FK → bookings |
| article_id | uuid | FK → articles |
| pickup_status | text | Nullable: picked_up, swapped, not_available |
| return_status | text | Nullable: returned_ok, delayed, broken, lost, pending |
| notes | text | |

### article_events
| Column | Type | Notes |
|---|---|---|
| id | uuid | PK |
| article_id | uuid | FK → articles |
| actor_id | text | FK → users |
| event_type | text | status_change, issue_reported, issue_resolved, booked, picked_up, returned, note |
| description | text | Human-readable summary |
| metadata | jsonb | Structured data (old/new status, booking_id, issue_id, etc.) |
| created_at | timestamptz | |

### audit_log
| Column | Type | Notes |
|---|---|---|
| id | uuid | PK |
| user_id | text | FK → users |
| action | text | e.g. 'booking.created', 'article.status_changed' |
| entity_type | text | e.g. 'booking', 'article' |
| entity_id | uuid | |
| details | jsonb | What changed |
| created_at | timestamptz | |

## Testing Strategy

Testing focuses on verifying that functionality works end-to-end, not on unit-testing individual functions.

### API integration tests (Go)

The primary test suite. Each test starts a real Postgres instance (via testcontainers-go), runs migrations, and exercises the API through HTTP requests.

Tests are organized as scenario tests — each tells a story and verifies outcomes through the API only (no peeking into the database to assert).

Each test uses a helper that starts a real Postgres (testcontainers-go), runs migrations, seeds test data, and provides an HTTP client with a fake JWT for a given role/unit. No mocks except for external services (notifications).

```go
func TestBookingFlow(t *testing.T) {
    env := setupTestEnv(t) // starts postgres, runs migrations, seeds data
    leader := env.ClientAs("leader", "yggdrasil")
    
    // ... test the full flow via HTTP calls to the API
}
```

#### Critical test scenarios

**Availability (highest risk — bugs here cause double-bookings):**
- `TestAvailability_NoDoubleBooking` — leader A books 3 tents for June 5-8, leader B books same type for June 7-10, only remaining tents are offered, leader B gets different articles assigned
- `TestAvailability_DelayedBlocksBooking` — article returned as delayed, another leader can't book that article until it's actually returned
- `TestAvailability_ReturnedArticleBecomesAvailable` — after full return, articles are bookable again

**Booking lifecycle:**
- `TestBookingFlow_FullLifecycle` — create → submit → confirm → pickup (tick all) → return (all OK), article statuses correct at each step
- `TestBookingFlow_PartialReturn` — pick up 5 items, leader A returns 3 (OK, delayed, broken), booking still open with 2 pending, leader B (same unit) returns rest, booking auto-completes
- `TestBookingFlow_SwapDuringPickup` — confirmed with article X assigned, swap for Y during pickup, X available again, Y loaned

**Approval:**
- `TestApproval_LeaderNeedsApproval` — leader books restricted item → submitted (not auto-confirmed) → manager approves → confirmed
- `TestApproval_ProjectLeaderSkipsApproval` — project leader books restricted item → auto-confirmed

**Access control (security):**
- `TestAccess_UnitBookingVisibility` — Yggdrasil leader creates unit booking, other Yggdrasil leader can manage it, Ornéerna leader cannot see it, equipment manager can see it
- `TestAccess_RoleEnforcement` — leader gets 403 on article CRUD and approval endpoints
- `TestMultiTenancy_GroupIsolation` — articles/bookings/issues in group A invisible to group B users

### Frontend E2E tests (Playwright)

A smaller suite that verifies critical user journeys through the actual UI:
- Login → browse → book → see confirmation
- Pickup checklist → tick items → swap one → confirm
- Return checklist → mark items → partial return
- Equipment manager: create article, handle issue report

Runs against the full stack (Go API + SvelteKit + Postgres) in Docker Compose. Uses a test Keycloak realm with preconfigured users.

### Dev mode: role switching

In development (`NODE_ENV=development`), the SvelteKit app includes a role switcher UI — a floating panel that lets developers switch between preconfigured personas without re-authenticating:

- **Leader (Yggdrasil)** — standard leader, member of unit "Yggdrasil"
- **Leader (Ornéerna)** — leader in a different unit, to test unit-scoped booking access
- **Project leader** — can book anything without approval
- **Equipment manager** — full admin access
- **Leader + Equipment manager** — combined roles

The switcher works by overriding the JWT claims in the SvelteKit server hooks (before they're passed to the Go API). The Go API also supports a `X-Dev-Role-Override` header in development mode that replaces the JWT claims with a preconfigured persona. This header is ignored in production.

This enables:
- Manual testing of role-based UI differences (what a leader sees vs an equipment manager)
- Verifying that unit-scoped bookings are visible/invisible to the right people
- Testing approval flows without switching browser sessions
- Playwright E2E tests that switch roles mid-test

The personas and their claims are defined in a shared config file (`dev-personas.json`) used by both SvelteKit and the Go API test helpers.

### What we don't do

- No unit tests for handlers, database queries, or utility functions unless they contain complex logic worth testing in isolation (e.g. availability calculation, article assignment algorithm).
- No mocking the database. Tests use real Postgres.
- No snapshot tests for UI components (we use @scouterna/ui-webc which is tested upstream).

### CI

All API integration tests run on every push. Playwright E2E tests run on PRs to main.

## Versioning and Releases

### Semantic versioning

The project uses [Semantic Versioning](https://semver.org/). The API and frontend are versioned together as a single product — one version number, one release.

- **Major** — breaking API changes (removed/renamed endpoints, changed response shapes)
- **Minor** — new features (new endpoints, new UI flows)
- **Patch** — bug fixes, styling, performance

### API versioning

The API is served under `/api/v0/*`. The version in the URL is the API contract version, not the release version.

- `/api/v0/*` is the current stable API
- When breaking changes are needed, introduce `/api/v2/*` and keep v1 running for a deprecation period
- Non-breaking additions (new fields, new endpoints) are added to the current version

### Release flow

Releases are managed by [Release Please](https://github.com/googleapis/release-please) with Conventional Commits.

1. All commits to `main` follow [Conventional Commits](https://www.conventionalcommits.org/) (`feat:`, `fix:`, `docs:`, etc.)
2. Release Please maintains a release PR that accumulates changes and bumps the version accordingly
3. When the release PR is merged:
   - A Git tag is created (e.g. `v1.3.0`)
   - A GitHub release is created with auto-generated changelog
   - CI builds Docker images tagged with the version and `latest`
   - Images are pushed to the container registry
4. Deployment to the VPS is triggered manually or by a webhook (pull new images, `docker compose up -d`)

### Branch strategy

- `main` — always deployable, protected
- Feature branches → PR → squash merge to `main`
- No develop branch, no release branches

### Docker image tags

- `ms-utrustning-api:v1.3.0` / `ms-utrustning-api:latest`
- `ms-utrustning-web:v1.3.0` / `ms-utrustning-web:latest`

### Seed data

The first migration includes a minimal default dataset: one group (Mälarscouterna), the initial locations (Kammaren, Östergården, Ladan, Kallförrådet, Hajkförrådet, Magasinet, Verkstan), and a single default category (Övrigt). Additional categories are auto-created during CSV import from the tags column. Everything is editable through the admin UI once real users are in the system.

### CI/CD

The repository is on GitHub. Builds are done by Jenkins:
- On push: run Go integration tests, lint
- On PR to main: run full test suite including Playwright E2E
- On release (Release Please merge): Jenkins builds Docker images, tags with version + `latest`, pushes to registry
- Deployment: manual trigger or webhook — pull new images on VPS, `docker compose up -d`

## Implementation Plan

### Phase 1 — Core booking loop

Each step produces a working, testable increment with both API and frontend. Steps 1–2 are the foundation, steps 3–6 are the booking loop, steps 7–8 round out Phase 1.

#### Step 1: API foundation ✅
- JWT auth middleware (validate token, extract claims, upsert user)
- Dev mode override (`X-Dev-Role-Override` header using dev-personas.json)
- sqlc config + first queries
- Error response helpers
- Test harness (testcontainers-go, fake JWT helper, HTTP client per role)

#### Step 2: Article CRUD + CSV import (equipment manager) ✅
- API: Create, read, update, delete articles. List with filtering (category, location, status, search).
- API: Location and category CRUD
- API: CSV import endpoint
- Integration tests: manager can CRUD, leader gets 403, CSV import creates correct articles

CSV column mapping:
| CSV column | DB field | Notes |
|---|---|---|
| titelgrupp | commercial_name | Product type (e.g. "Sibley", "Stormkök") |
| title | common_name | Individual item name (e.g. "Sibley 1") |
| description | description | |
| location | location_id | Resolved by name, "Karsvik" items use plats column |
| plats | location_id | Sub-location for Karsvik items (Ladan, Östergården, Kallförrådet) |
| rum + lage | place | Combined as free text |
| tags | category_id | Auto-created if not exists, normalized to title case |

#### Step 3: Article browsing (all users) ✅
- API: Public article list/search (read-only, scoped to group)
- Frontend: Browse page with category/location filters, search, grouped by product + location

#### Step 4: Booking — create & submit ✅
- API: Create draft booking, add/remove items, set used-by (unit/external/personal), submit
- Availability calculation (check status + overlapping bookings)
- Article assignment on confirm (pick specific articles from available pool)
- Frontend: Availability view, cart UI, date picker, booking list, booking detail, cancel, copy
- Integration tests: availability, no double-booking

#### Step 5: Booking — pickup checklist ✅
- API: Transition to picked_up, per-item pickup status, swap articles
- Frontend: Checklist view showing assigned articles + locations, tick off, swap
- Integration tests: pickup flow, swap during pickup

#### Step 6: Booking — return checklist ✅
- API: Per-item return status (OK/delayed/broken/lost), partial returns, explicit complete
- Delayed: item stays on loan, booking stays open, article remains unavailable for overlapping bookings
- Broken/lost: auto-create issue report, update article status
- Frontend: Return checklist with status options per item
- Integration tests: partial return, delayed blocks availability, broken creates issue

#### Step 7: Issue reporting (API ✅, frontend ✅)
- API: Create issue report (any user), list/resolve (manager), article event history ✅
- `article_events` table for per-article audit trail ✅
- Integration tests ✅
- Frontend: Report issue from article view, manager issue queue, article event history ✅
- Frontend: Role-aware issues page (leaders see read-only, managers get status controls) ✅

#### Step 8: Access control & multi-tenancy tests ✅
- Dev persona switcher (cookie-based, floating panel, clean path to real OIDC)
- Identity architecture: `User` type, `data.user` from layout, hooks proxy injects auth
- Unit-scoped booking visibility (Yggdrasil leader sees it, Ornéerna doesn't)
- Unit membership enforcement on booking creation (leaders can only book for own units)
- Projects as a unit type (`units.type = 'project'`), project leaders book for own projects
- Equipment managers see all bookings
- Role enforcement across all endpoints (leader gets 403 on manager endpoints)
- Article status role restrictions (leaders can report, managers can set any status)
- Group isolation test (two groups can't see each other's data)
- Pickup/return/swap event logging with acting user
- 10s polling on booking detail during active statuses for concurrent editing
- Integration tests: 6 test suites, 23 subtests

### Phase 2 — Approval + manager tools

#### Step 1: Equipment manager — inventory management
- Article create/edit form (all fields)
- Article list with bulk actions (status change, location move)
- CSV import UI with progress/error feedback
- Location and category CRUD pages

#### Step 2: Issue report images
- Image attachments on issue reports (upload + display)
- `issue_report_images` table, upload endpoint, local storage

#### Step 3: Approval flow
- API: Approve/reject bookings with restricted articles, manager approval queue endpoint
- Frontend: Manager approval queue, approve/reject with optional message
- Frontend: Leader sees "awaiting approval" status, gets feedback on rejection

### Phase 3 — Production auth + notifications

Connect real OIDC, add notifications, and make the system usable by actual users.

#### Step 1: OIDC authentication ✅
- Real JWT validation against ScoutID (Keycloak) JWKS endpoint in Go API using golang-jwt + keyfunc
- OIDC login flow in SvelteKit via @auth/sveltekit with Keycloak provider
- Scoutnet token claim mapping via `role-mapping.json` config:
  - `preferred_username` (`scoutnet|MEMBER_ID`) → member ID
  - `group:GROUP_ID:ROLE` → group ID + admin/project roles
  - `troop:TROOP_ID:ROLE` → leader role + unit membership
- Login page at `/login` with ScoutID branding, auto-redirects unauthenticated users
- User profile page at `/profile` showing roles and units grouped by access type
- Sign-out from profile page (clears Auth.js session)
- Dev persona switcher kept in dev mode, includes "ScoutID login" option
- Expired token detection — stale sessions trigger re-auth instead of 500s
- Dev seed script checks for dev mode before running

#### Step 2: Notifications
- Email notifications (approval requests, booking confirmations, overdue reminders)
- Google Chat webhook notifications (per-unit spaces, admin space)
- Per-user notification channel preference (email / Google Chat)

#### Step 3: Deployment + CI/CD
- CI/CD pipeline: Jenkins builds, Docker image tagging, push to registry
- Production Docker Compose with reverse proxy, TLS
- Deployment automation (webhook or manual trigger)
- User profile page (notification preferences)

### Phase 4 — Packages + polish
- Package CRUD (org-wide + personal)
- Package → cart flow
- Print-friendly checklist view
- Booking history per user
- Inventory dashboard (counts per location/status)

### Phase 5 — Reporting + scale
- Loan history reporting (per article, per person, per location)
- Article image management improvements
- Group onboarding (second scout group)
- Multi-tenancy hardening

## Internationalization (i18n)

The UI is internationalized from the start. Swedish (`sv`) is the default locale, English (`en`) is planned as the second language.

- All user-facing strings in the SvelteKit frontend go through an i18n system — no hardcoded Swedish in components.
- The Go API is language-agnostic: returns data as stored, uses error keys (not human-readable messages) so the frontend can translate them.
- User-generated content (article names, category names, descriptions) is stored as-is — not translated.
- Code, comments, API field names, and documentation are always in English.

## Open / TBD

- Overdue reminder schedule (daily? configurable?)
- Whether booking date granularity needs to go below day level in the future
- Token refresh — currently the access token from initial login is used until expiry, then the user is redirected to re-authenticate. Auth.js token rotation could be added to refresh tokens silently.
- Per-organisation role mapping — currently hardcoded in `role-mapping.json`, eventually needs to be configurable per group
