# Inventory Management — Phase 2 Step 2

Design and implementation plan for the equipment manager's inventory management tools.

## Goal

Give equipment managers a complete UI for managing their group's inventory: create/edit articles, bulk operations, CSV import/export, and group settings (locations, categories, notifications). Also add CSV export for the browse page and booking export on the bookings page, plus a print-friendly fetch list on booking detail.

## Scope

### In scope
1. **Manager mode toggle on browse page** — edit, bulk actions, archive, count field
2. **Article create/edit forms** — full form with all article fields
3. **CSV import on settings page** — upload with duplicate detection and feedback
4. **CSV export** — download current browse view as CSV
5. **Booking export** — download booking items as CSV
6. **Print-friendly fetch list** — on booking detail page
7. **Settings page** — user settings for all, group settings (locations, categories, import, notifications) for managers
8. **`group_settings` table** — per-group configuration with explicit columns, encrypted SMTP key

### Out of scope (deferred)
- Image upload (article model already has `image_path`, upload comes later)
- Role mapping UI (separate feature when onboarding second group)
- Rich text editing for descriptions (plain text for now)
- Docker secrets (Phase 3 — currently encryption key in `.env`)

## Design Decisions

### Enhanced browse page with manager mode toggle

The browse page and admin article list share the same data model, grouping, and expandability. Rather than a separate admin page, the browse page gains a **"Hanteringsläge" toggle** that shows/hides manager controls. The toggle is session-only state (a `$state` variable) — lost on page reload, no persistence needed.

**Toggle off (default):** Clean browse view, same as today for all users.

**Toggle on (managers only):**
- Checkboxes for bulk selection (per group, per article within individually tracked groups)
- Bulk actions toolbar (status change, location move, archive) — appears when items are selected
- Edit link per article/group
- Count field for quantity tracked groups (in expanded view)
- "Skapa artikel" button

The toggle is a session-only `$state` variable — no persistence, lost on full page reload. For SvelteKit client-side navigation (no reload), a writable store in `$lib/stores/` keeps it alive across page transitions.

### Separate pages

Only workflows that don't fit inline:
- `/articles/new` — create article form
- `/articles/[id]/edit` — edit individually tracked article

These pages are guarded server-side (manager only, redirects to `/browse`).

### Settings on profile page

Settings live on the profile page (`/profile`) as a second tab, not a separate page. Two tabs:

**Profil tab** (all users):
- Role cards with descriptions and unit badges
- "Mina inställningar" section (placeholder for future personal settings)
- Sign out

**Gruppinställningar tab** (managers only):
- Location CRUD (reorder, rename, add, delete)
- Category CRUD (reorder, rename, add, delete)
- CSV import (file upload with result display)
- Notification routing: webhook URL, notification email from-address, SMTP key

### Group settings — database table with explicit columns

```sql
CREATE TABLE group_settings (
    group_id text PRIMARY KEY REFERENCES groups(id),
    notification_email_from text NOT NULL DEFAULT '',
    smtp_key_encrypted bytea,
    gchat_webhook_url text NOT NULL DEFAULT '',
    default_approval_level text NOT NULL DEFAULT 'none',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
```

**Why explicit columns over jsonb:**
- Type safety — each setting has a proper type
- Default values handled by the DB
- No JSON migration problem — adding a new setting is a standard goose migration
- sqlc generates typed Go structs automatically

**SMTP key encryption:**
Each group can have their own SMTP API key (e.g. SendGrid, Mailgun) for sending from their own domain. The key is encrypted with AES-256-GCM before storage. The encryption key lives in `.env` as `SETTINGS_ENCRYPTION_KEY` — later migrates to a Docker secret at `/run/secrets/settings_encryption_key`. The API encrypts on write, decrypts on read. The frontend never sees the raw key — the settings endpoint returns a masked version (e.g. `"sk-...7f2a"`) and accepts a new key to replace it.

**Security boundary:**
- `group_settings` stores per-group config: webhook URL, email from-address, encrypted SMTP key, preferences
- `.env` stores: `SETTINGS_ENCRYPTION_KEY`, fallback SMTP config (for groups without their own key), system-wide settings
- Future: `SETTINGS_ENCRYPTION_KEY` moves to Docker secret. Per-group SMTP keys stay encrypted in DB.

### Article list — grouped and expandable

Same grouping as browse: articles grouped by `commercial_name + location`.

**Individually tracked groups** — expanding shows individual articles with:
- Status badge per article
- Edit link per article (manager mode)
- Checkbox per article for bulk actions (manager mode)
- Common name, place, status

**Quantity tracked groups** — expanding shows:
- Status summary (e.g. "42 OK, 3 under reparation, 2 saknas")
- Count field: number input showing current count, manager types desired count (manager mode)
- Edit link for group properties (manager mode)
- Group-level checkbox for bulk actions (manager mode)

### Count field for quantity tracked items

A number input where the manager types the desired total count. The system reconciles:
- **Increasing** (42 → 45): creates 3 new article records with status `ok`
- **Decreasing** (42 → 38): archives 4 records, preferring articles not in active bookings

One `count_changed` article event is logged on a representative article in the group (the first by creation date), with metadata `{ "old_count": 42, "new_count": 45 }`. This gives a clean inventory adjustment history — the manager sees "went from 42 to 45" rather than 3 individual creation events. The individual article records are still created/archived (the booking system needs them), they just don't each get their own event.

The count field submits on blur or Enter. No +/− buttons — direct input is faster for real adjustments.

Note: for quantity tracked items, "archive" and "delete" don't make conceptual sense as user-facing actions — the manager just changes the count. Internally the system archives excess records, but the UI presents it as a count adjustment.

### Article form

**Individually tracked**: Single form component for create and edit.

| Field | Input type | Required | Notes |
|---|---|---|---|
| commercial_name | text | No | Product type grouping |
| common_name | text | Yes | Individual item identifier |
| category_id | select | Yes | From group's categories |
| location_id | select | Yes | From group's locations |
| status | select | Yes | Default: ok |
| individually_tracked | checkbox | No | Default: true. Locked after creation if articles exist in bookings |
| approval_level | select | Yes | none/low/high, default: none |
| description | textarea | No | Plain text |
| instructions | textarea | No | Usage instructions |
| place | text | No | Where within location |
| purchase_date | date | No | |
| purchase_price | number | No | |

**Quantity tracked group edit**: Same fields minus `common_name` and `status`. Plus a `count` field.

### Archive and deletion workflow

**We don't delete, we archive.** Deletion is a second step only available on already-archived articles.

When archiving an article that's in an active booking (confirmed, approved, picked_up):
1. Check if an equivalent article exists (same `commercial_name` + `location`, bookable status, not in an overlapping booking)
2. If yes: auto-swap the archived article for the equivalent in all affected bookings, log the swap as a booking event
3. If no equivalent: return the conflict to the manager — show affected bookings that need manual resolution

For bulk archive: process each article, auto-replace where possible, collect conflicts, show the manager a summary.

**Deletion** is only available on archived articles, with a confirmation dialog.

### CSV import — two-phase with duplicate detection and revert

Lives on the settings page. The import is a two-phase process:

**Phase 1 — Preview (dry run):**
Manager uploads CSV. The API parses it and checks each row against existing articles (matching on `common_name + group_id`). Returns a preview without writing anything to the DB:

```json
{
  "rows": [
    { "row": 1, "common_name": "Sibley 1", "action": "create" },
    { "row": 2, "common_name": "Sibley 2", "action": "duplicate", "existing_id": "uuid", "existing_location": "Ladan", "existing_status": "ok" },
    { "row": 3, "common_name": "", "action": "skip", "reason": "missing title" }
  ],
  "summary": { "create": 45, "duplicate": 3, "skip": 2, "error": 1 }
}
```

The UI shows each row with its action. For duplicates, the existing article's details are shown so the manager can understand the conflict. The manager can then choose per-duplicate: skip or update existing.

**Phase 2 — Confirm:**
Manager reviews the preview and confirms. The request includes the manager's decisions on duplicates:

```json
{
  "mode": "confirmed",
  "duplicate_action": "skip",  // or "update" — applies to all duplicates
  // or per-row overrides if we need that granularity later
}
```

The API executes the import and returns the result including an `import_batch_id`.

**Revert:**
The confirm response includes an `import_batch_id`. Articles created by the import are tagged with this batch ID (new nullable column `import_batch_id` on articles). A "Ångra import" button calls `DELETE /articles/import/{batch_id}` which deletes all articles from that batch — but only if none of them have been booked. If any have been booked, the revert is blocked and the UI shows which articles can't be removed.

The revert option is shown on the settings page after a successful import. It remains available until the articles are modified or booked.

### CSV export

**Browse page export**: Downloads the current filtered article list as CSV. Columns match the import format for round-trip compatibility:
- title, titelgrupp, description, tags, location, rum, lage, count, requires_approval

Client-side from already-loaded data. Visible to all users. Export matches the current filter state.

**Booking export**: Downloads the booking's items as CSV:
- commercial_name, common_name, location, place, category, pickup_status, return_status

Client-side from already-loaded booking items data.

### Print-friendly fetch list

Print-optimized view of the booking's items, sorted by location then category. Triggered by a "Skriv ut hämtlista" button on booking detail (visible on confirmed/approved/picked_up bookings). Uses `@media print` CSS. Shows:
- Booking dates and unit
- Items grouped by location, sorted by category within each location
- Checkbox column for manual tick-off
- Article name, place (shelf/room), category

### Location and category deletion

Blocked if any articles reference the location/category. The API returns 409 with `{ "error": "has_articles", "count": N }`. The UI shows the count and tells the manager to reassign or archive articles first.

## API Changes

### New endpoints

**`PUT /articles/bulk`** (manager only) — Bulk update articles.

```json
// Request
{
  "article_ids": ["uuid1", "uuid2"],
  "status": "archived",
  "location_id": "uuid"
}

// Response
{
  "updated": 5,
  "conflicts": [
    {
      "article_id": "uuid3",
      "article_name": "Sibley 3",
      "booking_id": "uuid4",
      "booking_dates": "2026-06-05 — 2026-06-08",
      "booking_unit": "Yggdrasil",
      "replacement_available": false
    }
  ]
}
```

**`GET /group-settings`** (manager only) — Read group settings. SMTP key returned masked.

**`PUT /group-settings`** (manager only) — Update group settings. SMTP key encrypted before storage.

### Modified endpoints

**`POST /articles/import?mode=preview`** — Dry-run import. Parses CSV, checks for duplicates, returns per-row preview without writing to DB.

**`POST /articles/import?mode=confirmed`** — Execute import with manager's duplicate decisions. Returns result with `import_batch_id`.

**`DELETE /articles/import/{batch_id}`** — Revert an import batch. Deletes all articles with matching `import_batch_id`, blocked if any are booked.

**`DELETE /locations/{id}`** and **`DELETE /categories/{id}`** — Return 409 with `{ "error": "has_articles", "count": N }` when articles reference them.

### New migration

Single migration `00010_group_settings.sql` combining both changes:

```sql
-- +goose Up
CREATE TABLE group_settings (
    group_id text PRIMARY KEY REFERENCES groups(id),
    notification_email_from text NOT NULL DEFAULT '',
    smtp_key_encrypted bytea,
    gchat_webhook_url text NOT NULL DEFAULT '',
    default_approval_level text NOT NULL DEFAULT 'none',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT group_settings_approval_check CHECK (default_approval_level IN ('none', 'low', 'high'))
);

ALTER TABLE articles ADD COLUMN import_batch_id uuid;
CREATE INDEX idx_articles_import_batch ON articles(import_batch_id) WHERE import_batch_id IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_articles_import_batch;
ALTER TABLE articles DROP COLUMN IF EXISTS import_batch_id;
DROP TABLE IF EXISTS group_settings;
```

## Implementation Plan

### Step 2a: Database + settings page foundation ✅

1. ✅ Migration `00010_group_settings.sql`: `group_settings` table + `import_batch_id` column on articles (combined in one migration)
2. ✅ sqlc queries for group settings CRUD + article count per location/category
3. ✅ AES-256-GCM encryption helper (`api/internal/crypto/encrypt.go`) reading `SETTINGS_ENCRYPTION_KEY` from env
4. ✅ `GET /group-settings` and `PUT /group-settings` API endpoints with SMTP key encryption/masking
5. ✅ Settings as "Gruppinställningar" tab on profile page (not a separate page)
6. ✅ Location CRUD UI with article count check on delete (409 with count)
7. ✅ Category CRUD UI (same pattern)
8. ✅ Basic CSV import section on profile page (uses existing import endpoint)
9. ✅ `DELETE /locations/{id}` and `DELETE /categories/{id}` return 409 with `{ "error": "has_articles", "count": N }`
10. ✅ `ApiError` class in client.ts preserving full response body for richer error messages
11. Deferred to Step 2c: two-phase import (preview → confirm → revert) — basic import works now

**UPDATE**: Settings page was originally planned as a separate `/settings` route. Changed to a tab on the profile page for mobile-first simplicity. `default_approval_level` setting is locked to `none` (no UI).

### Step 2b: Article create/edit forms ✅

1. ✅ Shared `ArticleForm.svelte` component — handles both create and edit modes
   - Create mode: count field + individually tracked toggle. If individually tracked, shows editable name list pre-filled with `{commercial_name} {N}`. If quantity tracked, creates N records with same name.
   - Edit mode: single article, all fields
2. ✅ `/articles/new` page (manager-guarded) — supports `?from={commercial_name}&location={location_id}` for pre-filling from existing article group
3. ✅ `/articles/[id]/edit` page (manager-guarded) — permanent delete only for archived articles
4. ✅ `/articles/[id]` detail page (all users) — read-only view with description, instructions, status, report issue, event history. Manager notes (amber background) visible only to managers. "Redigera" link for managers.
5. ✅ Added `purchase_date` and `purchase_price` to API handler's `articleRequest` struct (were in DB/query but not wired through)
6. ✅ Added `manager_notes` field — migration `00011_manager_notes.sql`, sqlc queries updated, API handler updated, form field visible only to managers
7. ✅ Updated Article interface in client.ts with `instructions`, `purchase_date`, `purchase_price`, `import_batch_id`, `manager_notes`

**UPDATE**: Routes use `/articles/*` instead of `/admin/articles/*` — no separate admin namespace. Article detail page added as a natural read-only view for all users, complementing (not replacing) the browse page expand.

### Step 2c: Browse page manager mode ✅ (partial)

1. ✅ Article detail links: individually tracked articles get pill-button links on common_name in expanded view, quantity tracked groups get "Visa artikelsida ›" link
2. ✅ Edit links per article/group: "Redigera ›" pill-button links (manager only) — individually tracked → `/articles/{id}/edit`, quantity tracked → `/articles/{id}/edit?group=true`
3. ✅ "Skapa artikel" button (manager mode)
4. ✅ `PUT /articles/bulk` endpoint with archive conflict detection + auto-replacement
5. ✅ `POST /articles/group-count` endpoint — atomic count adjustment, logs single `count_changed` event, protects representative (oldest)
6. ✅ `PUT /articles/{id}?group=true` — applies shared fields to all articles in quantity tracked group
7. ✅ Auto-propagation of shared fields on individually tracked article save (description, instructions, manager_notes, category_id)
8. ✅ Article detail page: quantity tracked shows status summary, aggregated purchase info, collapsed group events
9. ✅ `GET /articles/{id}/group-events` — aggregated event history across all articles in a group
10. ✅ Edit form: three layouts — individually tracked (shared/per-item sections), quantity tracked (single blue box with count), create (flat)
11. ✅ Shared fields: description, instructions, manager_notes, category_id. Per-item: common_name, status, approval_level, location_id, place, purchase_date, purchase_price
12. ✅ Name validation warning when common_name doesn't start with commercial_name
13. ✅ CSV import reads `instructions` and `manager_notes` columns
14. ✅ Example CSV enriched with descriptions, instructions, manager_notes, rum/lage
15. ✅ Consistent link styling: pill-button with › for navigation, underline for in-page actions
16. ✅ "Materialare" → "Utrustningsansvarig" terminology
17. ✅ "Hanteringsläge" toggle (session state, manager only) — partially wired
18. ✅ Checkboxes (per group and per article) — state management done, UI partially wired
19. ✅ Expandable description/instructions/manager notes on browse page ("Visa info" toggle)
20. ✅ Count field for quantity tracked groups in browse expanded view (manager only)
21. ✅ Article note input on detail page (`POST /articles/{id}/events` with `note` type)
22. ✅ Per-physical-item list on group edit page (expandable, shows status/purchase per item)
23. ✅ Event collapsing excludes notes (comments always show individually)
24. ✅ Report issue form responsive fix (wraps on narrow screens)
25. ✅ "Anteckning" → "Kommentar" terminology
26. Remaining: bulk actions toolbar UI (status change, location move, archive dropdowns)

**UPDATE**: Step 2c scope expanded significantly from original plan. Article detail page gained quantity tracked group support (status summary, aggregated purchase info, collapsed group events). Edit form split into three layouts with shared/per-item field distinction. Shared field propagation added for individually tracked articles. Approval level moved from shared to per-item (different locations may need different approval rules). Location is per-item (same product type can exist in multiple locations).

### Step 2d: CSV export + booking export + print fetch list

1. Add CSV export button to browse page (client-side, all users)
2. Add CSV export button to booking detail page (client-side)
3. Add print-friendly fetch list to booking detail page
4. Print styles via `@media print` CSS

### Step 2e: Integration tests

1. Test bulk article operations (status change, location move, archive with replacement)
2. Test archive conflict detection (article in active booking, no replacement available)
3. Test CSV import preview (dry run returns correct duplicate/create/skip per row)
4. Test CSV import confirm + revert (batch created, revert deletes, revert blocked if booked)
5. Test location/category deletion blocked by articles
6. Test group settings read/write (including SMTP key encryption round-trip)
7. Test admin access control
8. Test count field changes (creates/archives records, logs single count_changed event)

### Step 2f: Documentation

1. Update `docs/API.md` with new/modified endpoints
2. Update `docs/SPEC.md` — mark Phase 2 Step 2 progress
3. Update `docs/guide.md` — add inventory management and settings to user guide
4. Update `docs/BACKLOG.md` — remove completed items
5. Update `docs/accomplished.md` — log completed work

## Order of implementation

```
2a → 2b → 2c → 2d → 2e → 2f
```

2a (settings page with location/category CRUD + import) is the foundation. 2b (article forms) needs locations and categories. 2c (browse manager mode) needs the article forms to link to. 2d (exports + print) is independent.
