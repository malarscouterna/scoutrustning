# Accomplished

A living log of completed work — what was built, when, and why. Major features and revamps reference their own docs files. Smaller completions are written directly here.

When finishing a backlog item or spec milestone, log it here and remove it from the backlog / mark it done in the spec.

Newest first.

---

## 2026-04-09

### Inventory management foundation (Phase 2 Step 2a+2b)

Built the foundation for equipment manager inventory management. See [inventory-management.md](inventory-management.md) for the full design doc.

**Group settings**: `group_settings` table with per-group notification config (email from-address, encrypted SMTP key via AES-256-GCM, Google Chat webhook URL). GET/PUT endpoints, manager-only. Settings UI as "Gruppinställningar" tab on the profile page.

**Location/category CRUD UI**: Reusable `CrudList.svelte` component. Deletion blocked when articles reference the entity (409 with count).

**Article create/edit forms**: Shared `ArticleForm.svelte` with create and edit modes. Create mode supports multi-article creation (count field + editable name list for individually tracked, count-only for quantity tracked). Pre-fill from existing article group via `?from=` query param. New `manager_notes` field (private, amber-highlighted, manager-only). Added `purchase_date` and `purchase_price` to API handlers.

**Article detail page**: `/articles/[id]` read-only view for all users — description, instructions, status, report issue, event history. Manager notes visible only to managers. Edit link for managers.

**CSV import**: Basic import UI on profile page settings tab (uses existing API endpoint).

**API client**: `ApiError` class preserving full response body for richer error messages (e.g. article count on 409).

**Svelte 5 cleanup**: Resolved all pre-existing `state_referenced_locally` warnings and a11y label issues across browse, book, bookings, and return checklist pages. Zero svelte-check warnings. Added rules to prevent regressions.

### Integration tests for inventory management

`TestInventoryManagement` with 6 subtests: group settings CRUD, leader access denied, location/category delete blocked by articles, empty location deletable, article with manager_notes and purchase fields.

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

## 2026-04-10

### Image upload infrastructure and display (Phase 2 Step 3, partial)

See [images.md](images.md) for the full design doc with 7-step implementation plan.

**Image processing pipeline**: `api/internal/images/` package using govips (libvips CGO wrapper). Accepts JPEG, PNG, WebP, HEIC up to 25MB. Strips all EXIF metadata, auto-rotates, center-crops to 4:3 for product images (no crop for issue images). Produces two WebP variants: source (1920px longest edge, q80) and thumbnail (400×300, q70). On-demand JPEG conversion for download via `?format=jpeg` query parameter.

**MIME detection**: Byte-level content sniffing — `http.DetectContentType` for JPEG/PNG, RIFF header check for WebP, ftyp box brand check for HEIC/HEIF. Robust against missing or incorrect Content-Type headers in multipart uploads.

**API endpoints**: `POST /images/product` (manager-only), `POST /images/issue` (any user), `GET /images/{uuid}.webp` and `GET /images/{uuid}_thumb.webp` (serve with immutable cache), `DELETE /images/product` (manager-only). Product upload propagates `image_path` to all articles sharing `commercial_name + location_id + group_id`. Replace deletes old files. Delete clears `image_path` on all matching articles.

**Docker changes**: API Dockerfile adds `gcc musl-dev vips-dev` (build) and `vips` (runtime). `images` Docker volume in `docker-compose.yml`, local `./data/images` mount in dev override. `IMAGE_DIR` env var.

**Frontend display**: `image_path` field added to `Article` TypeScript interface. Browse page shows thumbnail in expanded info section ("Visa info" toggle). Article detail page shows thumbnail at top. Both use `ImageViewer` component — tap opens a `<dialog>`-based lightbox with full-resolution image and "Ladda ner" (JPEG download) link.

**Seed script**: Uploads images from `docs/seed-images/` directory, mapping filenames to commercial names (e.g. `sibley.webp` → "Sibley", `stormkok.jpg` → "Stormkök" at both locations). Clears orphaned image files on re-seed.

**Integration tests**: `TestImageUpload` with 8 subtests: leader cannot upload product image, manager uploads product image (files on disk + image_path on articles), serve WebP source, serve WebP thumbnail, serve JPEG download, 404 for nonexistent, replace deletes old files, delete clears image_path, leader can upload issue image.

**sqlc queries**: `UpdateArticleGroupImagePath`, `ClearArticleGroupImagePath`, `GetArticleGroupImagePath`.

**Test infrastructure**: `BaseURL()` method on `TestClient` for building custom multipart requests. `libvips-dev` documented as test prerequisite.

### Browse page manager mode + article detail enhancements (Phase 2 Step 2c, partial)

See [inventory-management.md](inventory-management.md) for the full design doc.

**Browse page links and edit buttons**: All users see article links in expanded view — individually tracked articles get pill-button links on common_name, quantity tracked groups get "Visa artikelsida ›" link. Managers see "Redigera ›" buttons alongside. Consistent link styling: pill-button with › chevron for navigation, underline text for in-page actions (Rapportera, Historik).

**Manager mode toggle**: "Hanteringsläge" checkbox (session state, manager only) with "Skapa artikel" button. Checkboxes per group and per article for bulk selection (state management done, toolbar UI remaining).

**API — bulk operations**:
- `PUT /articles/bulk` — bulk status change, location move, archive with conflict detection + auto-replacement in active bookings
- `POST /articles/group-count` — atomic count adjustment for quantity tracked groups, logs single `count_changed` event, protects representative (oldest article never archived)
- `PUT /articles/{id}?group=true` — applies shared fields to all articles in a quantity tracked group
- `GET /articles/{id}/group-events` — aggregated event history across all articles in a group

**Article detail page — quantity tracked support**: Detects quantity tracked groups and shows status summary (e.g. "42 OK, 3 under reparation"), aggregated purchase info (unique dates/prices across group), and collapsed group events (consecutive same-type events within 60s shown as "Bokad ×3"). Report issue available for all article types.

**Edit form — three layouts**:
- Individually tracked: blue "Gemensamt" section (shared fields) + neutral "Enskild artikel" section (per-item fields)
- Quantity tracked (`?group=true`): single blue box with all fields + count
- Create: flat layout (unchanged)

**Shared field propagation**: Saving an individually tracked article auto-propagates description, instructions, manager_notes, and category_id to all siblings with the same commercial_name. Approval level and location are per-item (different locations may need different approval rules).

**Name validation**: Warning when common_name doesn't start with commercial_name.

**CSV import**: Now reads `instructions` and `manager_notes` columns. Example CSV enriched with realistic descriptions, instructions, manager notes, and rum/lage data.

**Terminology**: "Materialare" → "Utrustningsansvarig" across all UI.

**Cleanup**: Extracted shared `$lib/labels.ts` module — `statusLabels`, `statusColors`, `approvalLabels`, `eventTypeLabels`, `eventTypeColors` — replacing duplicated constants across browse, article detail, issues, and ArticleEventHistory. Removed unused SQL queries (`GetOldestArticleInGroup`, `ListArticlesByGroup`). Removed unused `onCountChange` prop.

**Bug fix**: Quantity tracked count change from edit page wasn't working — `goto('/browse')` in `onSubmit` aborted the async chain before `onCountChange` ran. Fixed by consolidating the full save sequence (update article + count change + navigate) in the edit page's `handleSubmit`.

**Migration**: `00012_count_changed_event.sql` — adds `count_changed` to the `article_events` event_type check constraint.

**Integration tests**: `TestBrowseManagerMode` with 9 subtests: bulk status change, bulk location move, leader access denied, group count increase (with event verification), group count decrease, group update applies to all, shared field propagation (including approval_level NOT propagating), group events aggregation, count change via edit page flow, leader access denied on group-count.

**Browse page — expandable info**: "Visa info ▼" toggle in expanded view shows description, instructions, and manager notes (amber box, manager only). Shared across both individually and quantity tracked groups via Svelte 5 render snippet.

**Browse page — inline count field**: Quantity tracked groups show an "Antal" number input in the expanded view (manager only). Submits on change via the group-count API.

**Article detail — comment input**: Text input + "Spara" button above the history section. Calls `POST /articles/{id}/events` (new endpoint, returns 204). History refreshes immediately after adding (key-based re-render for individually tracked, explicit reload for quantity tracked). Notes excluded from event collapsing — comments always show individually.

**Group edit — per-physical-item list**: Expandable "Visa enskilda artiklar (N st)" section below the form showing each physical item with status badge, purchase date, and purchase price.

**Fixes**: Report issue form wraps on narrow screens. "Anteckning" → "Kommentar" in event type labels and input placeholder.

**Bulk actions toolbar**: Manager mode toolbar with status change (ok/under repair/lost/archive), location move, and approval level dropdowns. Comment input appears for status/location changes. Per-article checkboxes in individually tracked expanded view. Archive option hidden when quantity tracked groups are selected (use count field on edit page instead). Events logged with comment for all bulk operations.

**Browse page cleanup**: "Visa arkiverade" moved to manager-only row with "Hanteringsläge". Badge shows available/nonArchived count (archived items excluded). Removed inline count field from browse — count changes only on edit page where per-item list is visible. "Inköpspris per styck" label on create form.

**Backlog additions**: View-only access tier, smarter count decrease on edit page, duplicate article name checking on create, per-item editing on count increase.
