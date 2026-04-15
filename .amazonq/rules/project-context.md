# ms-utrustning - Project Context

## Purpose

Reference material for AI assistance. Describes what the project is, how it's built, and key design decisions. For coding rules see `coding-conventions.md`, for interaction expectations see `workflow.md`.

## What this project is

An equipment booking service for scout groups. Leaders book scouting equipment (tents, stoves, knives, etc.), pick it up with a checklist, and return it. Equipment managers maintain inventory and handle issue reports.

The full specification is in `docs/SPEC.md` - read it before making architectural decisions.

## Architecture

- **Go API** (Chi v5 + pgx v5 + sqlc + govips) - JSON REST API at `/api/v0/*` (pre-release, breaking changes allowed without version bump)
- **SvelteKit 2 frontend** - Svelte 5, responsive web app, mobile-first for leaders, uses `@scouterna/ui-webc` web components and `@scouterna/tailwind-theme`
- **PostgreSQL 17** - single database, all tables scoped by `group_id` for multi-tenancy
- **Docker Compose** - Go API + SvelteKit + Postgres, behind a reverse proxy

Auth: SvelteKit handles OIDC login with ScoutID (Keycloak) via Auth.js (`@auth/sveltekit`). Go API validates JWTs and extracts claims. No user registration - users are upserted from token claims on login.

## Project structure

```
ms-utrustning/
├── api/                    # Go API
│   ├── cmd/server/         # main.go entrypoint
│   ├── internal/
│   │   ├── auth/           # JWT validation middleware + OIDC claim parsing
│   │   ├── crypto/         # AES-256-GCM encryption for sensitive settings
│   │   ├── handler/        # HTTP handlers per resource
│   │   ├── db/             # sqlc generated code + queries
│   │   ├── images/         # Image upload, processing (govips), serving
│   │   └── notify/         # Email + Google Chat notifications
│   ├── migrations/         # goose SQL migration files
│   ├── sqlc.yaml
│   ├── go.mod
│   └── Dockerfile
├── web/                    # SvelteKit frontend
│   ├── src/
│   │   ├── lib/
│   │   │   ├── api/client.ts       # Typed API client
│   │   │   ├── components/         # Shared Svelte components (BookingCard, IssueCard, FloatingCart, ...)
│   │   │   ├── stores/cart.svelte  # Active booking cart store (localStorage key active-booking-id)
│   │   │   └── user.ts             # User type + role helpers (accessAtLeast, isManager, canBook, ...)
│   │   ├── routes/
│   │   │   ├── +layout.server.ts   # Provides user + dev persona data
│   │   │   ├── +layout.svelte      # Sticky top bar (logo + Hem/Bokningar links), FloatingCart, DevPersonaSwitcher
│   │   │   ├── +page.svelte        # Dashboard: CTAs, bookings sections, issues sections, footer
│   │   │   └── +page.server.ts     # Dashboard data (bookings, issues, pending approval count)
│   │   ├── hooks.server.ts         # API proxy, auth header injection
│   │   └── app.html
│   ├── static/
│   ├── package.json
│   ├── svelte.config.js
│   └── Dockerfile
├── docs/
│   ├── SPEC.md             # Full specification (living document)
│   ├── API.md              # API reference (keep updated)
│   ├── BACKLOG.md          # Open work items
│   ├── accomplished.md     # Completed work log
│   ├── issues-and-events.md
│   ├── article-status-refactor.md
│   └── guide.md            # User-facing guide (shown in UI)
├── .amazonq/rules/
│   ├── project-context.md  # This file - reference
│   ├── coding-conventions.md # How to write code
│   └── workflow.md         # How to interact with the user
├── dev-personas.json       # Dev personas for role switching
├── docker-compose.yml
└── README.md
```

## Key identity decisions

- `groups.id` is `text`, using the Keycloak org ID directly (e.g. `"766"` for Mälarscouterna)
- `users.id` is `text`, using the Keycloak member ID directly (e.g. `"3000924"`)
- All other tables use `uuid` primary keys
- Units are a managed table (`teams`), not free text - populated from OIDC claims or created by admins. The `teams` table has a `type` column (`troop` or `role`) to distinguish scout troops from functional roles. Each team has a configurable `access_level` (view, book, trusted, manager).
- Booking approval is per-article (`articles.approval_level`). Three levels: `none` (freely bookable), `low` (trusted teams auto-approve, book-level teams need manager approval), `high` (always needs manager approval, including managers themselves). If any item in a booking requires approval for the current user, the whole booking waits.

## Article model

- `commercial_name` is the product type (e.g. "Sibley", "Stormkök") - what users browse and book by.
- `common_name` is the individual item identifier (e.g. "Sibley 1") - for physical identification at pickup.
- Availability is grouped by `commercial_name + location`. Same product in different locations shows as separate groups.
- `approval_level` is set per article (`none`, `low`, `high`). During CSV import, read from the `requires_approval` column; defaults to `none` if absent.
- Quantity-tracked items: each physical unit is a separate row with `individually_tracked = false`. Manager sets count via UI after import.
- Article `status` represents **condition** (ok, reported_usable, incoming, reported_unusable, under_repair, lost, archived) - orthogonal to booking state (reserved, loaned out), which is computed from booking data.
- `expected_available_date` (nullable) is used with `incoming` and `under_repair` statuses. Articles with these statuses become bookable for date ranges starting on or after this date.
- Availability is always computed at query time from article condition + booking overlaps - never stored as a column.

## Identity & auth architecture

The frontend has a clean separation between "who is the user" and "where does that info come from":

- `$lib/user.ts` defines the `User` type and helpers like `hasRole()`. This is the stable interface all pages and components consume.
- `+layout.server.ts` provides `data.user` (the current user) to all pages. In production this comes from the OIDC session; in dev mode from the persona cookie. Pages never care about the source.
- `hooks.server.ts` proxies all `/api/*` requests to the Go API and injects the auth header. In dev mode: reads the `dev-persona` cookie and sets `X-Dev-Role-Override`. In production: forwards the real Bearer token.
- **No page or component should ever reference personas, cookies, or auth headers directly.** They consume `data.user` from the layout and call `createApiClient()` without auth options - the hooks layer handles identity injection.
- OIDC claims (`group:766:it_manager`, `troop:17443:vice_leader`) are mapped to teams via `team_claim_mappings` in the database. Each team has a configurable access level (view, book, trusted, manager). Teams are auto-created on first login or pre-created by managers in the settings UI. The `init-group` CLI bootstraps the first group and manager team.

## Scalability

Expected upper bounds per group: ~5,000 articles, ~20 concurrent overlapping bookings. The current single-Postgres + Go API architecture requires no changes for this domain.

Availability queries (`AvailableArticles`, `AvailableArticlesExcludingBooking`, `ListArticlesWithAvailability`) only scan bookings that overlap the requested date range (`b.start_date <= @end_date AND b.end_date >= @start_date`). Historical bookings (returned, cancelled, or outside the window) are excluded via `idx_bookings_dates`. At 20 concurrent bookings the `NOT IN` subquery touches ~200–400 booking_item rows - trivial for Postgres. Total booking history does not affect query cost.

If pagination is ever needed on article lists, that's a query-level change, not architectural.

## Environment modes

All differences between dev, demo, and production are controlled via `.env`. One `docker-compose.yml` for all modes.

| Variable | Dev | Demo | Production |
|---|---|---|---|
| `DEV_MODE` | `true` | `true` | `false` |
| `DEMO_MODE` | `false` | `true` | `false` |
| `BUILD_TARGET` | `dev` | `production` | `production` |

- **Dev**: hot reload, persona switcher, auto-fallback to default persona (no login required), Postgres exposed
- **Demo**: production builds, OIDC login required (ScoutID), persona switcher available after login, demo banner shown, Postgres not exposed
- **Production**: production builds, OIDC login required, no persona switcher, no demo banner

Demo mode requires `DEV_MODE=true` (for persona switcher) but gates access behind OIDC - no auto-persona fallback without logging in first.

## Security practices

- All API endpoints require a valid JWT except health checks.
- Role checks in Go middleware: equipment manager endpoints reject non-managers.
- `group_id` is derived from the JWT, never from request parameters.
- File uploads (images) are validated for type and size, stored outside the web root.
- SQL injection is prevented by sqlc's parameterized queries.
- In demo/production: `POSTGRES_PORT` is not exposed. Password is randomly generated.
- `AUTH_SECRET` and `POSTGRES_PASSWORD` are randomly generated in demo/production.
- The SvelteKit proxy **strips** `X-Dev-Role-Override` and `Authorization` headers from incoming browser requests before forwarding to the Go API. Identity is injected server-side only.
- The Go API port (8080) is bound to `127.0.0.1` - only reachable from the host machine. Only the SvelteKit port (3000) should be exposed via the reverse proxy.

## Version pinning

Check `api/go.mod` for Go dependency versions and `web/package.json` for frontend dependency versions. Notable constraints:
- TypeScript must be ^5.x (not 6.x) - SvelteKit requires ^5.3.3
- PostgreSQL 17.x

## Development environment

Tool paths (may not be on default PATH):

| Tool | Path |
|---|---|
| Go | `/usr/local/go/bin/go` |
| Node | `/usr/local/bin/node` |
| pnpm | On PATH via snap or `corepack enable` |
| sqlc | `$HOME/go/bin/sqlc` (`go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest`) |
| Docker | `/usr/bin/docker` |
| libvips | System package (`sudo apt install libvips-dev`) - required for image processing tests |

When running Go commands, ensure PATH includes Go:
```bash
export PATH=$PATH:/usr/local/go/bin:$HOME/go/bin
```
