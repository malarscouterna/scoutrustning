# Issues Revamp - First-class Issue Entities

## Summary

Promote issue reports from article-status markers to standalone entities with their own URLs, lifecycle, assignees, and comment threads. Add a dedicated report-issues page reachable from the dashboard. Turn the inline issue-reporting shorthand (booking return, browse, article detail) into pre-filled entry points to the same flow. Keep `/issues` as the list view; add `/issues/[id]` as the detail page.

## Goals

- Issues are addressable, linkable, and persistent - each one has a URL
- One issue can reference multiple articles
- Multiple independent issues can be open on one article at the same time
- Article status is derived from open issues, not set explicitly
- Any user can report, comment on, and follow an issue; managers can change status and assign
- Assignments surface the issue on the assignee's dashboard
- Issue list and dashboard always show created and last-updated dates
- The reporting UX is consistent: the same slide-up form is used everywhere (browse, article page, booking return, check-out)

## Non-goals (deferred)

- Notifications (email/push) - architecture not yet in place; assignment is a placeholder until then
- Multi-article selection UX in the report form - start with one article, refine after launch
- Migrating historical issue data from article_events - old events remain readable as history

---

## Data model

### New table: `issue_reports`

```sql
CREATE TABLE issue_reports (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    group_id    text NOT NULL REFERENCES groups(id),
    title       text NOT NULL,
    description text NOT NULL,
    severity    text NOT NULL CHECK (severity IN ('usable', 'unusable', 'missing')),
    status      text NOT NULL DEFAULT 'open'
                  CHECK (status IN ('open', 'in_progress', 'resolved', 'archived')),
    reporter_id text NOT NULL REFERENCES users(id),
    booking_id  uuid REFERENCES bookings(id),  -- set when reported via booking flow
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);
```

`status` values:
| Value | Meaning |
|---|---|
| `open` | Newly reported, no manager action yet |
| `in_progress` | Manager has acknowledged and is working on it |
| `resolved` | Problem fixed, articles back to OK |
| `archived` | Issue closed without resolution (e.g. article retired) |

`severity` maps directly to article status:
| Severity | Article status |
|---|---|
| `usable` | `reported_usable` |
| `unusable` | `reported_unusable` |
| `missing` | `reported_missing` |

### New table: `issue_articles`

Links issues to one or more articles.

```sql
CREATE TABLE issue_articles (
    issue_id    uuid NOT NULL REFERENCES issue_reports(id) ON DELETE CASCADE,
    article_id  uuid NOT NULL REFERENCES articles(id),
    group_id    text NOT NULL REFERENCES groups(id),
    PRIMARY KEY (issue_id, article_id)
);
```

### New table: `issue_assignees`

```sql
CREATE TABLE issue_assignees (
    issue_id    uuid NOT NULL REFERENCES issue_reports(id) ON DELETE CASCADE,
    user_id     text NOT NULL REFERENCES users(id),
    group_id    text NOT NULL REFERENCES groups(id),
    assigned_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (issue_id, user_id)
);
```

### New table: `issue_events`

Activity log per issue (comments, status changes, assignments, image attachments).

```sql
CREATE TABLE issue_events (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    issue_id    uuid NOT NULL REFERENCES issue_reports(id) ON DELETE CASCADE,
    group_id    text NOT NULL REFERENCES groups(id),
    actor_id    text NOT NULL REFERENCES users(id),
    event_type  text NOT NULL
                  CHECK (event_type IN ('comment', 'status_change', 'assignment', 'article_added', 'article_removed')),
    description text NOT NULL DEFAULT '',
    metadata    jsonb NOT NULL DEFAULT '{}',  -- { image_ids?, new_status?, user_id? }
    created_at  timestamptz NOT NULL DEFAULT now()
);
```

### Article status derivation

`articles.status` continues to be a stored column (backward-compatible with all existing queries and availability logic). It is updated by application code whenever an issue on that article is created, resolved, or archived:

| Open issues on the article | Derived status |
|---|---|
| None | `ok` (or whatever it was before any issue) |
| At least one `missing` | `reported_missing` |
| At least one `unusable` (no `missing`) | `reported_unusable` |
| Only `usable` | `reported_usable` |

Severity priority for derivation: `missing` = `unusable` > `usable` > `ok`. Both `missing` and `unusable` block booking; they are peers at the highest severity level. If an article has both open `missing` and `unusable` issues, `reported_missing` takes precedence (arbitrary tiebreak - can be revisited).

Rules:
- When a new issue is created for an article: set article status to `reported_{severity}` if it is worse than the current status (missing = unusable > usable > ok).
- When an issue is resolved or archived: re-derive from remaining open issues; if none remain, restore to `ok`.
- When an article is archived (`articles.status = 'archived'`): set all open issues on that article to `archived`.
- An issue that spans multiple articles updates all of them.

The existing `PUT /api/v0/articles/{id}/status` endpoint is kept for internal and manager-only use cases (`under_repair`, `incoming`, `archived`) that are not tied to issue entities. The `lost` article status is removed - articles confirmed gone should be set to `archived`.

---

## Pages

### Dashboard (`/`)

**Issues section** - mirrors the bookings section layout:

```
Ärenden
─────────────────────────────────────────
  [Felanmälan ›]             ← primary CTA → /issues/new
─────────────────────────────────────────
  Trasig tältpinne · Sibley 6p           ← /issues/[id]
  Ej användbar · rapporterat 12 apr, uppdaterat idag ›

  Spricka i kastrull · Stormkök          ← /issues/[id]
  Användbar · rapporterat 3 apr ›
─────────────────────────────────────────
  [Visa alla ärenden ›]                  → /issues
```

**"Mina ärenden"** (non-manager): issues the user reported or is assigned to, open only. Badge shows count of open "mine" issues.
**"Aktiva ärenden"** (manager): 5 most recent open/in_progress issues across the group. Two badges: count of "mine" (assigned/reported) + total open count (like pending approvals). Highlighted if total > 0.

Both views show: issue title, severity label, article name(s), created date, last-updated date.

---

### Issues list (`/issues`)

Same structure as today but built from `issue_reports` rows instead of article status filters.

```
Ärenden
─────────────────────────────────────────
  [Rapportera ett problem ›]             ← primary CTA → /issues/new

Mina ärenden
  [issue card]  ...
  [issue card]  ...

Övriga ärenden              (manager only)
  [issue card]  ...
  [issue card]  ...

  [Visa avslutade ▼]
─────────────────────────────────────────
```

**Issue card** - same component for dashboard and list:
- Title
- Severity badge ("Användbar" / "Ej användbar" / "Saknas"), amber/red/red
- Article name(s), linked to `/articles/[id]`
- Reporter name
- Dates: "Rapporterat 12 apr · Uppdaterat idag"
- Status badge if not open (In progress, Resolved, Archived)
- Assignee avatar(s) if any
- Tap/click → `/issues/[id]`

---

### Report issue page (`/issues/new`)

Also reachable with query params for pre-filling from shorthands: `/issues/new?article_id=...&severity=unusable&booking_id=...`

```
Rapportera ett problem
─────────────────────────────────────────
Utrustning
  [search input: type to find articles]
  [selected article chip(s) with ×]

Allvarlighetsgrad
  ( ) Användbar - kan fortfarande användas
  (•) Ej användbar - kan inte användas
  ( ) Saknas - finns inte där den ska finnas

Severity values: `usable`, `unusable`, `missing`

Beskrivning *
  [textarea - required]

Bilder (valfritt)
  [image upload]

  [Skicka rapport ›]
─────────────────────────────────────────
```

- Article search: inline autocomplete against `/api/v0/articles` (name, commercial name). Single selection for now; multi-article is deferred.
- Severity defaults to `unusable` when pre-filled from a booking return.
- **Title is auto-generated** on the API from article name + severity, e.g. "Sibley 6p - Ej användbar". Not shown as a field in the form. Editable on the detail page after creation.
- On submit: creates issue, redirects to `/issues/[id]`.
- Pre-fill params (`article_id`, `severity`, `booking_id`) are applied on load; the user can still edit before submitting.

---

### Issue detail page (`/issues/[id]`)

```
← Ärenden

[Title]                              [Status badge]
Ej användbar · rapporterat av Anna K · 12 apr 2026

Utrustning
  [Sibley 6p - Hajkförrådet]  →  /articles/[id]   ← scout-list-view-item, chevron

Tilldelad
  [+ Lägg till]  Anna K ×   Björn S ×

─────────────────────────────────────────
Händelser
  12 apr  Anna K rapporterade problemet
          "Tältpinnen är trasig, kan inte sätta upp tältet"
          [image thumbnail]

  13 apr  Björn S: "Tittar på det"   ← comment
          Status ändrad till Pågår

  [text input: Skriv en kommentar ...]   [Skicka]
─────────────────────────────────────────

(manager only)
  [Markera som löst ›]   [Arkivera ›]   [Pågår ›]
```

**Available to all authenticated users:**
- View all fields and event history
- Add a comment (creates `issue_events` row with type `comment`)
- Attach images to a comment
- The issue is added to "Mina ärenden" on their dashboard once they comment

**Available to managers (`issue_resolve_role`):**
- Change status: open → in_progress → resolved / archived
- Add/remove assignees (any group member)
- Add/remove linked articles

**Status change creates an `issue_events` row** with type `status_change`, optional comment, and `metadata.new_status`.

**Assignment creates an `issue_events` row** with type `assignment`, `metadata.user_id`.

**When resolved:** all linked articles have their status re-derived (may return to `ok` if no other open issues remain).

**When archived:** all linked articles that have no other open issues return to `ok`; articles that are archived keep their status.

---

## Issue reporting shorthand

A reusable `ReportIssueSheet.svelte` component used everywhere: browse article expand, article detail page (`/articles/[id]`), and booking return checklist.

**Trigger**: a "Rapportera problem" button on any article row/detail. Tapping it slides up a bottom sheet.

**Sheet content (pre-filled where context is available):**
```
Rapportera problem med [Article Name]

Allvarlighetsgrad
  ( ) Användbar
  (•) Ej användbar
  ( ) Saknas

Beskrivning *
  [textarea]

Bilder (valfritt)
  [upload]

  [Skicka]  [Avbryt]
```

**On submit:**
1. Calls `POST /api/v0/issues` with `article_id`, `severity`, `description`, `booking_id` (if in booking context).
2. Sheet closes.
3. Toast appears: "Problem rapporterat. [Visa ärende →]" linking to `/issues/[id]`.
4. Article status updates reactively (derived from new open issue).

**Booking return flow:** selecting `reported_usable`, `reported_unusable`, or `missing` as the return status opens the sheet pre-filled with that severity. The `lost` booking item return status is removed - a missing item is reported as a `missing` severity issue instead. The booking item return and the issue creation happen as two separate API calls; the return call no longer sets article status directly (status derives from the issue instead).

**No navigation away.** The user stays on their current page throughout.

---

## API changes

### New endpoints

| Method | Path | Auth | Description |
|---|---|---|---|
| `POST` | `/api/v0/issues` | Any user | Create an issue |
| `GET` | `/api/v0/issues` | Any user | List issues (filters: `status`, `mine`, `article_id`) |
| `GET` | `/api/v0/issues/{id}` | Any user | Get issue detail + events + assignees + articles |
| `PUT` | `/api/v0/issues/{id}` | Manager for status; any for title/desc | Update issue fields |
| `POST` | `/api/v0/issues/{id}/comments` | Any user | Add a comment (with optional images) |
| `PUT` | `/api/v0/issues/{id}/assignees` | Manager | Replace assignee list |
| `POST` | `/api/v0/issues/{id}/articles` | Manager | Add an article to the issue |
| `DELETE` | `/api/v0/issues/{id}/articles/{articleId}` | Manager | Remove an article from the issue |

### Existing endpoints - behavior changes

| Endpoint | Change |
|---|---|
| `PUT /api/v0/articles/{id}/status` | Restricted to manager-only status transitions not tied to issues: `under_repair`, `incoming`, `archived`. The `lost` status is removed - use `archived` instead. Reporting (`reported_*`) is no longer done via this endpoint. |
| `PUT /api/v0/bookings/{id}/items/{itemId}/return` | When return status is `reported_usable`, `reported_unusable`, or `missing`, no longer sets article status directly - caller is expected to create an issue via `POST /api/v0/issues` and pass the `booking_id`. Article status derives from the issue. The `lost` return status is removed. |

### Response shape - issue detail

```json
{
  "id": "uuid",
  "title": "string",
  "description": "string",
  "severity": "usable | unusable",
  "status": "open | in_progress | resolved | archived",
  "reporter": { "id": "string", "name": "string" },
  "booking_id": "uuid | null",
  "articles": [
    { "id": "uuid", "commercial_name": "string", "location_name": "string" }
  ],
  "assignees": [
    { "id": "string", "name": "string", "assigned_at": "timestamptz" }
  ],
  "events": [
    {
      "id": "uuid",
      "actor": { "id": "string", "name": "string" },
      "event_type": "comment | status_change | assignment | article_added | article_removed",
      "description": "string",
      "metadata": {},
      "created_at": "timestamptz"
    }
  ],
  "created_at": "timestamptz",
  "updated_at": "timestamptz"
}
```

---

## Migration notes

This revamp intentionally breaks backwards compatibility. The system is pre-release (`/api/v0`), so no migration shims are needed.

- **`PUT /articles/{id}/status` for reported statuses is removed.** The endpoint remains for manager-only transitions (`under_repair`, `incoming`, `archived`). Any client calling it with `reported_usable`, `reported_unusable`, or `missing` gets a 400. The `lost` article status is removed entirely - use `archived` for confirmed-gone articles.
- **`lost` booking item return status is removed.** Clients sending `lost` as a return status get a 400. Use `missing` severity when creating an issue for an item that cannot be found.
- **Existing `article_events` rows** (type `issue_reported`, `issue_resolved`) are not migrated to `issue_events`. They remain readable as historical context on `/articles/[id]` event history.
- **`lost` article status removed**: the value is dropped from the article status CHECK constraint. No data migration - pre-release.
- **Articles currently in `reported_usable`/`reported_unusable` status**: no back-fill. Their status stays as-is. Managers resolve them via the issues page as before; once resolved the status derives from issue entities going forward. The new `/issues` list shows only `issue_reports` rows - old reported articles appear only on `/articles/[id]` history until manually resolved.
- `issues-and-events.md` is superseded by this document. Add a redirect notice to that file.

---

## Route changes

| Route | Change |
|---|---|
| `/issues` | Rebuilt to list `issue_reports` entities instead of article-status filter |
| `/issues/new` | New: report issue form with article search |
| `/issues/[id]` | New: issue detail with events, assignees, status actions |
| `/articles/[id]` | Add "Rapportera problem" button → `ReportIssueSheet` |
| `/browse` | Add "Rapportera problem" on article expand → `ReportIssueSheet` |
| `/bookings/[id]` | Return flow: selecting broken status opens `ReportIssueSheet` instead of setting status directly |
| `/` (dashboard) | Add "Rapportera ett problem" CTA + issue cards with dates |

---

## File changes

### New
| File | Purpose |
|---|---|
| `api/migrations/NNNN_issue_reports.sql` | New tables: issue_reports, issue_articles, issue_assignees, issue_events |
| `api/internal/db/queries/issues.sql` | sqlc queries for issues |
| `api/internal/handler/issues.go` | Issue handlers |
| `web/src/routes/issues/[id]/+page.svelte` | Issue detail page |
| `web/src/routes/issues/[id]/+page.server.ts` | Server load for issue detail |
| `web/src/routes/issues/new/+page.svelte` | Report issue form |
| `web/src/routes/issues/new/+page.server.ts` | Server actions for report form |
| `web/src/lib/components/ReportIssueSheet.svelte` | Reusable slide-up report form |
| `web/src/lib/components/IssueCard.svelte` | Issue card for list + dashboard |

### Modified
| File | Change |
|---|---|
| `api/internal/handler/articles.go` | Restrict `UpdateStatus` to non-reported statuses; add status-derivation logic |
| `api/internal/handler/bookings.go` | Remove status-setting from return flow |
| `web/src/routes/issues/+page.svelte` | Rebuild around issue entities |
| `web/src/routes/issues/+page.server.ts` | Load from issues API |
| `web/src/routes/+page.svelte` | Add CTA + issue cards with dates |
| `web/src/routes/+page.server.ts` | Fetch issues for dashboard |
| `web/src/routes/browse/+page.svelte` | Add ReportIssueSheet trigger on article expand |
| `web/src/routes/articles/[id]/+page.svelte` | Add ReportIssueSheet trigger |
| `web/src/routes/bookings/[id]/+page.svelte` | Return flow uses ReportIssueSheet |
| `web/src/lib/api/client.ts` | New API client methods for issues |
| `smoke-test.sh` | Add checks for /issues/new and a fixture /issues/[id] |

---

## Implementation order

1. ✅ DB migration: `issue_reports`, `issue_articles`, `issue_assignees`, `issue_events` — `api/migrations/00002_issue_reports.sql`. Also removes `lost` from `articles.status`, adds `reported_missing`; removes `lost` from `booking_items.pickup_status`; replaces `lost` with `missing` in `booking_items.return_status`.
2. ✅ sqlc queries + regenerated — `api/internal/db/queries/issues.sql`
3. ✅ Issue handlers — `api/internal/handler/issues.go`. All endpoints implemented. Article status derivation runs on issue create/resolve/archive.
4. ✅ `PUT /articles/{id}/status` restricted to manager-only non-reported statuses (`ok`, `incoming`, `under_repair`, `archived`). `lost` removed. `reported_*` now only set via issue entities.
5. ✅ Booking return handler updated: `lost` removed, `missing` added, `reported_*` return statuses no longer set article status directly (caller creates issue via POST /issues).
6. ✅ `IssueHandler` mounted in `main.go` at `/api/v0/issues`.
7. ✅ API tests fully updated. `issues_test.go` rewritten. `access_test.go`, `browse_manager_test.go`, `images_test.go` updated. `pickup_test.go`: removed `lost` pickup_status sub-tests, replaced with `lost_pickup_status_is_rejected` (expects 400) and updated assertions. `view_only_test.go`: `can_report_issue` now uses `POST /api/v0/issues` with issues route mounted. `UpdateItemPickup` handler cleaned up — removed stale `article_status`/`lost` logic (pickup-time issue reporting now done via `POST /api/v0/issues`). All tests pass.
8. ⬜ `/issues/new` page + server action
9. ⬜ `/issues/[id]` detail page
10. ⬜ Rebuild `/issues` list page from issue entities
11. ⬜ `ReportIssueSheet` component
12. ⬜ Wire sheet into browse, article detail, booking return
13. ⬜ Dashboard: CTA + issue cards with dates
14. ⬜ Smoke test additions, svelte-check

---

## Resolved decisions

| # | Decision |
|---|---|
| 1 | **Article status derivation**: keep as a stored column, updated by application code. Easier queries, backward-compatible with all existing availability and filter logic. |
| 2 | **Multi-article UX**: start with single-article selection; extend later. |
| 3 | **Dashboard issue badges**: two counts, like pending approvals - one for "mine" (reported or assigned) and one for total open (manager only). |
| 4 | **Issue title**: auto-generated from article name + severity on creation (e.g. "Sibley 6p - Ej användbar"). Reduces friction in shorthand flows. Editable on the detail page. |
| 5 | **Old reported_* articles**: no back-fill migration. Pre-release, so backwards compatibility is intentionally broken. Old article status stays until a manager resolves it manually or a new issue is filed. The old `PUT /articles/{id}/status` path for reporting (`reported_*`) is removed. |

---

## Documentation updates

### Files to update

| File | What to change |
|---|---|
| `docs/SPEC.md` | Update "Issue Reports" section (currently says "no separate issue table"). Add UPDATE note to Step 7. Update return checklist flow ("Broken/Lost auto-creates issue report" - now creates an issue_reports row). Add new tables to Data Model section. Update Equipment manager: Issues user flow. |
| `docs/issues-and-events.md` | Mark as superseded by this document. Add a one-line redirect: "This design has been superseded by [issues-revamp.md](issues-revamp.md)." |
| `docs/BACKLOG.md` | Strike out / mark resolved: "Report issue - standalone entry point", "Issue reporting - rethink and browser entry points", "Ärenden - per-user filtering", "Issue reporting - booking context in event history". Update "Quantity-tracked items - issue reporting during pickup" to reference new ReportIssueSheet. Remove dead-code note from PickupChecklist.svelte item. |
| `docs/API.md` | Add new issue endpoints section. Update articles status endpoint restrictions. Update booking return endpoint (no longer sets article status). |
| `docs/accomplished.md` | Log this revamp as completed once implemented. |
| `.amazonq/rules/project-context.md` | Update project structure (new routes, new tables). Update issue reporting description. |
| `smoke-test.sh` | Add checks for `/issues/new` and a seeded `/issues/[id]`. |

### SPEC.md changes in detail

**Issue Reports section** (currently lines ~125-129):
```
# Before
There is no separate issue table. An article with a reported status *is* an open issue.

# After (UPDATE note)
UPDATE: Issues are now first-class entities in the `issue_reports` table.
See docs/issues-revamp.md for the full design.
```

**Data Model section**: add `issue_reports`, `issue_articles`, `issue_assignees`, `issue_events` tables.

**Step 7** in Phase 1:
```
UPDATE: The article-status-only model has been superseded. Issues are now
stored as issue_reports rows. See docs/issues-revamp.md.
```

**Return checklist** ("Broken/Lost auto-creates issue report"):
```
UPDATE: Selecting Broken/Lost in the return flow opens ReportIssueSheet
pre-filled with severity. The issue is created as an issue_reports row.
Article status is derived from open issues, not set directly.
```

### Backlog items superseded by this revamp

These items in `BACKLOG.md` should be struck out once implemented:

- **"Report issue - standalone entry point"** - fully covered by `/issues/new` + `ReportIssueSheet`
- **"Issue reporting - rethink and browser entry points"** - fully covered
- **"Ärenden - per-user filtering"** - covered by "Mina ärenden" section (reported by me + assigned to me)
- **"Issue reporting - booking context in event history"** - covered by `issue_reports.booking_id` FK
- **"Issues overview: show date issue was reported and date of last update"** (ux-revamp.md open item #14) - covered by `created_at`/`updated_at` on issue entities

These items are **partially addressed** and should be updated to note what remains:

- **"Quantity-tracked items - batch issue reporting"** - `ReportIssueSheet` covers single-article reporting from the group detail page; multi-item batch flow (count input for N articles) is still deferred
- **"Quantity-tracked items - issue reporting during pickup"** - dead code in `PickupChecklist.svelte` (`startGroupReport`, `confirmGroupReport`, `reportingGroupKey`) can now be wired to `ReportIssueSheet`; interaction model (does reporting reduce pickup count?) is still TBD
