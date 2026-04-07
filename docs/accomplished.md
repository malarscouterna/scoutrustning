# Accomplished

A living log of completed work — what was built, when, and why. Major features and revamps reference their own docs files. Smaller completions are written directly here.

When finishing a backlog item or spec milestone, log it here and remove it from the backlog / mark it done in the spec.

Newest first.

---

## 2026-04-07

### Mobile responsiveness overhaul

Fixed horizontal overflow and layout issues across all pages on phone-sized screens. Replaced tables with flex-wrap layouts, made rows and buttons wrap properly, fixed book page sticky bar hidden behind mobile nav. Moved persona switcher to top-right on mobile with scroll support. Removed redundant `manager-it` persona. Fixed user guide not loading by mounting `docs/guide.md` into web container.

### Friendly error for unconfigured groups

Users logging in via ScoutID from a group not in `role-mapping.json` previously got a 500 error (FK violation in UpsertUserMiddleware). Fixed:
- Go API: `UpsertUserMiddleware` detects the group_id FK violation and returns 403 with `"group_not_found"` error key
- Frontend: `parseUserFromSession` now returns `oidcName` when the user is authenticated but their group isn't mapped
- Layout shows a friendly Swedish message: "Din scoutkår är inte konfigurerad" with different text for demo (points to persona switcher) vs production (contact equipment manager)
- Unmapped users are redirected to `/` to prevent page load functions from crashing
- Added `X-Dev-Claims` header support in auth middleware for testing arbitrary claims
- Integration test: `TestAuth_UnknownGroupReturns403`

---

## 2026-04-06

### Demo mode, env generator, and deployment hardening

Built a complete deployment story: `gen-env.sh` generates `.env` for dev/demo/prod modes. Demo mode requires OIDC login but keeps the persona switcher for testing. Production strips all dev features. API port bound to localhost only, SvelteKit proxy strips auth headers from browser requests.

### Swedish usage guide

Added `/guide` page rendering `docs/guide.md` — covers browsing, booking, pickup/return, issue reporting, and role differences. Serves as demo walkthrough in demo mode.

### Mobile layout with bottom navigation bar

Replaced top-only navigation with a responsive bottom bar for mobile. Desktop keeps the sidebar.

### v0.2.0 released

Second release via Release Please. Includes approval flow, demo mode, deployment hardening, and guide.

---

## 2026-04-05

### Three-level approval flow with booking event history

Replaced the boolean `requires_approval` with a three-level model (`none`, `low`, `high`) per article. Project leaders auto-approve `low` items, managers auto-approve everything. Booking events table tracks the full approval conversation (submit → reject with message → resubmit → approve). See spec Phase 2 Step 1.

### Article status refactor — separate condition from booking state

Removed statuses that duplicated booking state (`booked`, `loaned`, `drying`, `new`). Article status now purely represents condition. Availability is always computed from booking data. Added `expected_available_date` for `incoming` and `under_repair` articles. See [article-status-refactor.md](article-status-refactor.md).

### Browse page availability display

Browse page now shows computed availability per article: reserved/loaned indicators with booking info, expected dates for incoming/under repair items. Header counts reflect real-time availability.

### CSV import count column

CSV import supports a `count` column — rows with count > 1 create multiple quantity-tracked articles from a single row.

### Event history limit, draft cleanup, pickup revert

- Article event history supports `?limit=N` with "show all" in the UI
- Background goroutine cleans up empty draft bookings after 48 hours
- Undoing all pickups reverts booking to pre-pickup status via `pre_pickup_status` column

### Shared test container

All integration tests share a single Postgres container via `TestMain` — each test truncates and reseeds for isolation. Significant speedup.

### Hot reload in Docker

Added `air` for Go API hot reload and Vite HMR for SvelteKit, both working inside Docker Compose.

---

## 2026-04-04

### OIDC authentication with ScoutID

Real JWT validation against Keycloak JWKS in Go API. SvelteKit handles OIDC login via `@auth/sveltekit`. Role mapping from Scoutnet token claims via `role-mapping.json`. Dev persona switcher kept alongside real auth. See spec Phase 3 Step 1.

### Access control, dev persona switcher, and unit/project model

Full role-based access control across all endpoints. Units and projects stored in same table with `type` column. Dev persona switcher with cookie-based role switching. Unit-scoped booking visibility. 6 test suites, 23 subtests.

### Issue reporting via article events

Replaced separate `issue_reports` table with article status + `article_events` audit trail. Any user can report, managers resolve by changing status. See [issues-and-events.md](issues-and-events.md).

### Pickup checklist and return flow

Per-item pickup with swap support. Return flow with status per item (OK/delayed/broken/lost). Partial returns keep booking open. Broken/lost auto-creates issue reports. Quantity-tracked items grouped in UI.

### CI/CD setup

Release Please for automated versioning + changelog. GitHub Actions for Docker image builds. v0.1.0 released.

---

## 2026-04-03

### Initial build — from scaffold to working booking loop

Built the entire Phase 1 in one session:
- Project scaffold (Go API + SvelteKit + Postgres in Docker Compose)
- API foundation: JWT auth middleware, sqlc, test harness with testcontainers-go
- Article CRUD + CSV import from the real Mälarscouterna inventory spreadsheet
- Article browsing with category/location/search filters, grouped by product + location
- Booking flow: create draft, add items with availability checking, submit, cancel, copy
- Location-scoped availability with double-booking prevention
- Booking UI with cart, date picker, booking list, booking detail
