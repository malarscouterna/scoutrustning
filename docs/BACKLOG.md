# Backlog

Deferred work items - things to grab when there's time, smaller tasks set aside during major work. When an item is completed, move it to [accomplished.md](accomplished.md).

## English user guide

`docs/guide.md` is the source for the in-app guide page and is currently written in Swedish only. An English version is needed once the English UI is fully rolled out. Deferred — low priority until there are actual English-speaking users.

## Group members API + assignee picker

Add `GET /api/v0/users` (manager only) that returns all users who have ever logged in to the group. Used by:
1. The issue detail page assignee picker - managers can add/remove assignees by selecting group members.
2. A new "Medlemmar" section on the group settings page so managers can see who has accounts in the group.

In demo mode (`DEMO_MODE=true`), the endpoint should return the configured personas from `dev-personas.json` instead of real users, so demo environments do not expose real member data.

Response shape: `[{ "id": "string", "name": "string", "email": "string" }]`

## Auth redirect loop - remove debug logging

`hooks.server.ts` logs `[auth] redirect to login` and `[auth] stale session cookie detected` on every unauthenticated request. Once the fix has been confirmed stable in production, remove these `console.log` calls or gate them behind a `DEBUG_AUTH` env flag.

## Tailwind theme integration - dynamic class support

Several badge/status colors are currently hardcoded as inline styles because Tailwind v4 + `@scouterna/tailwind-theme` only includes classes that appear as static strings at build time. Dynamic class composition (e.g. `class={badgeCls}`) gets purged. Affected: approval level badges in browse. Fix: either safelist the relevant color classes in the Tailwind config, or investigate whether the `@scouterna/tailwind-theme` package exposes CSS custom properties that can be used for inline color values instead.

## ~~Admin UI for group/role management~~ → Access levels

**Superseded by the access levels feature** (see [access-levels.md](access-levels.md)). The static `role-mapping.json` is being replaced by DB-driven `team_claim_mappings` + per-team access levels. The `init-group` CLI bootstraps groups, and managers configure teams/access via the settings UI.

Remaining from the original item:
- Auto-register groups/troops/roles when users attempt to log in → implemented as auto-discovery in auth middleware
- Admin interface for access levels → being built as kanban UI on settings page
- Replace "project" concept → done, replaced with `role` team type

## Mention users in issue comments

Ability to add another user (manager or trusted) to an issue by mentioning them in a comment. Requires `GET /api/v0/users` (blocked on Group members API above) and notification infrastructure.

## Date change conflict UX

When changing dates on a booking, the API validates that all existing items are available for the new range and returns 409 with the conflicting article name. Currently the UI just shows a red error. Better UX: highlight which items conflict and let the user remove them before retrying the date change.

## Copy booking

Copying a booking needs availability checking against the new dates. Ties into the package/kit feature (Phase 4) since both involve populating a booking from a template. The API endpoint (`POST /api/v0/bookings/{id}/copy`) exists but the UI is deferred.

## Booking editing - change tracking via booking_events

The `booking_events` table supports mutation tracking event types (`items_changed`, `dates_changed`, `details_changed`) with a `metadata` jsonb column. Currently only approval-flow events and `note` events are logged. Next steps:

1. Log `items_changed` events in AddItems/RemoveItem handlers
2. Log `dates_changed` events in Update handler
3. Log `details_changed` for unit/notes changes
4. Show these in the booking event thread
5. Eventually: "revert this change" button

## Stale drafts with items

Empty draft cleanup is implemented (48h). Stale drafts *with* items need notifications before deletion - users should be warned before their booking is removed.

## Quantity-tracked items - manager UI

**Partially built in Phase 2 Step 2c** (see [inventory-management.md](inventory-management.md)).

Done: count field on group edit page, group-count API endpoint, group events, article detail page with status summary.

Remaining:
- Count field inline on browse page expanded view (manager mode)
- Bulk actions toolbar UI on browse page (status change, location move, archive dropdowns)
- Per-physical-article list/edit UI for quantity tracked groups (edit individual purchase dates, prices)

## Subcategories

Categories are flat. The `categories` table supports `parent_id` but the UI doesn't use it yet.

## Date picker locale

Native `<input type="date">` uses browser/OS locale for week start day. Full control requires a custom date picker component.

## Unavailable items in copied bookings

When copying a booking, items that aren't available for the new dates should be visually marked, not silently included.

## Race conditions on concurrent edits

No optimistic locking. Booking detail polls every 10s during active statuses. Consider `updated_at` checks on writes if conflicts become a real problem.

## Manager article swap on active bookings

Managers should be able to swap articles on `confirmed`/`approved` bookings, not just during pickup (`picked_up` status). Use cases: unexpected unavailability, delayed returns from other bookings, inventory rebalancing. When an article is given a new status, managers should be informed about potential unavailability issues.

## Delayed return - conflict resolution

When a delayed article overlaps with another booking:
- Auto-swap for a free equivalent if possible
- Alert manager if no equivalent available

Currently shows a warning but doesn't resolve the conflict.

## Date validation and overdue handling

- Overdue bookings (`end_date` in past, still `picked_up`) - visual warning, manager notification
- Delayed return date in the past - validate or warn
- Booking start date in the past - allow for retroactive bookings?
- Overdue reminder schedule (daily? configurable?)

## Quantity-tracked items - return flow

Grouped return UI for quantity-tracked items: one row per product group with count inputs instead of individual rows with identical names.


## Pickup date validation

Only allow pickup on or after the booking's start date. Browse/inventory should distinguish "reserved for today but not yet picked up" from "currently checked out".

## Browse page - date picker for time-travel view

Add a date picker to view inventory state at any date. The API already supports a `date` param.

## Seed script - date sprawl

Backdate article events in the seed script for realistic history spread. No API changes needed.

## Booking flow - date change with items in cart

When dates change on a booking with items, re-validate all items against the new range. Currently the API returns 409 for conflicts but the UI doesn't handle it gracefully. The date fields should be locked once items are added to the cart - changing dates could invalidate the entire cart (articles no longer available). To change dates, the user should remove all items first, or the UI should show which items conflict and offer to remove them. Until this is built, date inputs are disabled after the first item is added.


## Shared visual identity for interactive elements

Audit and standardize the visual language for interactive elements across the app:
- **Navigation links** (opens a new page): pill-button with › chevron. Blue for primary (article name, "Visa artikelsida"), neutral for secondary ("Redigera"). Currently used on browse and article detail pages - verify consistency everywhere.
- **In-page actions** (toggles something on the current page): underline text. Blue for primary ("Rapportera"), neutral for secondary ("Historik", "Avbryt").
- **Manager-only elements**: how to visually distinguish manager controls from user controls? Currently relies on conditional rendering only - no visual indicator that something is a manager feature.
- **Bulk vs individual actions**: bulk toolbar vs per-item links. Need clear visual distinction when manager mode is fully wired.
- **Destructive actions**: red underline text ("Ta bort artikel permanent"). Consistent?
- **Primary action buttons**: filled blue ("Spara", "Skapa artikel"). Consistent sizing and placement?

Extract shared button/link component or at least document the pattern in coding-conventions.md.

## Article comments - delete own recent comments

After adding a comment on the article detail page, the user should be able to delete it (at least for a short time, e.g. within 5 minutes or until someone else adds an event). Needs a `DELETE /articles/{id}/events/{event_id}` endpoint with ownership + time check.

## Article history management for managers

On the article edit page, managers should be able to manage the article's event history: edit descriptions, delete erroneous events, add backdated notes. This is an admin tool for correcting mistakes, not a user-facing feature.

## Quantity-tracked items - per-item editing on count increase

When increasing count on a quantity tracked group, new articles get default per-item fields (status: ok, no purchase date/price). The expandable per-physical-item list on the group edit page should allow inline editing of these fields (purchase_date, purchase_price, status) per item. Could also support setting defaults for new items (e.g. "all new items purchased today at X kr").

## ~~Three-tier access: view-only, booking, managing~~ → Access levels

**Superseded by the access levels feature** (see [access-levels.md](access-levels.md)). The four access levels (view, book, trusted, manager) cover this and more. Per-team configuration replaces the hardcoded three tiers.

## Quantity-tracked items - smarter count decrease on edit page

When decreasing count on the group edit page, the system currently archives the newest non-booked articles. This is too blunt - the manager may want to archive a specific article (e.g. the broken one, not the newest one). The per-physical-item list on the edit page should allow the manager to select which specific articles to archive when decreasing count, rather than auto-selecting. Could be checkboxes on the item list with an "Arkivera markerade" action.

## Duplicate article name checking on create

When creating articles, there's no check for duplicate `common_name` within the group. Creating two "Sibley 1" articles would cause confusion. The API should check for existing articles with the same `common_name + group_id` and return a warning (not a hard block - the manager may intentionally want duplicates across locations). The frontend should show the warning and let the manager confirm.


## Article groups normalization

The concept of a "product group" (articles sharing `commercial_name + location_id`) is implicit today - enforced by convention and propagation logic. Several features depend on this grouping:

- Shared fields (description, instructions, manager_notes, category_id) propagated on save
- Product images (per group)
- Availability grouping in browse/booking
- Quantity tracked count management
- Group events aggregation

A proper `article_groups` table would make this explicit:

```
article_groups (
  id uuid PK,
  group_id text FK → groups,
  commercial_name text,
  location_id uuid FK → locations,
  category_id uuid FK → categories,
  description text,
  instructions text,
  manager_notes text,
  approval_level text,  -- default for new articles in the group
  created_at timestamptz
)
```

Articles would get an `article_group_id` FK instead of duplicating shared fields. The `product_images` table (currently keyed on `group_id + commercial_name + location_id`) would re-key to `article_group_id`.

**Benefits**: single source of truth for shared fields, no propagation logic, cleaner FKs for images/packages/future features, simpler queries.

**Cost**: large refactor touching most article queries and handlers. Migration must create groups from existing articles and backfill FKs.

**Current approach**: `product_images` uses the composite key `(group_id, commercial_name, location_id)` - designed to be easily re-keyed to `article_group_id` later without structural changes. The table has its own UUID PK so all references (frontend, other tables) use the UUID and survive the re-keying. Migration path: add `article_group_id` column, backfill from composite key match, drop the three columns.


## ~~Availability filter - access-level-aware "show locked items"~~ → Done

Implemented in the access levels feature. Approval badges on the availability picker now reflect the selected team's access level: `low`-approval items show "Kräver godkännande" only for `view`/`book` teams (trusted+ auto-confirms). `high` always shows the badge. Badges update reactively when the team picker changes.

## Browse - sort and filter by approval level

Add a visible sort/filter for approval level in the browse view. The default view should surface items the current user is pre-approved for first (none for book+, none+low for trusted+), then low-approval items, then high-approval items. Requires knowing the user's effective access level and sorting server-side or client-side. Currently approval level is shown as a badge per group row (no/amber/red). Proper sorting and filtering is the next step.

## Bokningar page - sort by status then start date

Bookings list should be sorted: submitted first, then approved/confirmed, then picked_up, then draft, then returned/rejected/cancelled. Within each status group, sort ascending by start_date. Currently the list is unsorted (server return order).


## Group settings on tab bar

The group settings page has too many fields to fit comfortably. Consider a compact layout where the most important fields are always visible on the tab bar area, with a "Fler inställningar" expander for the rest. Needs design thinking about which fields are most-used (locations, categories) vs rarely changed (SMTP key, webhook URL, image upload role). Possibly a two-column layout on wider screens with sections collapsed by default on mobile.

## Quantity-tracked items - issue reporting during pickup

Quantity groups are now split by status category at pickup: ok items show the count picker, reported_usable items show their issue detail and "Hämtad ändå" / "Ta bort från bokning" options, and reported_unusable items are shown disabled. The "Felanmäl" button on ok rows opens the ReportIssueSheet.

What's still missing: when confirming a count lower than booked, prompt the user to report the shortfall as missing items. Currently the shortfall is silently left with no pickup_status.

## Pickup/return - svelte-check Node compatibility

`svelte-check` now installed as a devDependency (`^3.x`). However, `@sveltejs/vite-plugin-svelte@7` uses `styleText` from `node:util` (added in Node 20.12), which causes svelte-check to fail when loading `svelte.config.js` on Node 18. Pre-existing TS errors in `BookingItemsList.svelte` and `+page.svelte` (object literal lookup pattern `{{key: val}[expr]}` is parsed incorrectly by svelte-check 3's TS engine) are also unresolved.

Fix options: upgrade to Node 20 on the host, or downgrade `@sveltejs/vite-plugin-svelte` to 6.x (requires verifying compatibility with SvelteKit 2 / Svelte 5).

## Pickup/return - AddItemSheet: non-approved item tap → new booking

When a non-approved item is tapped in `AddItemSheet`, a small inline hint shows "Kräver godkännande - skapa en ny bokning". The link to `/browse` is shown but no pre-fill of article or dates is passed. Could pre-fill the browse page with the cart's dates and the selected article as a search query for a smoother handoff.

## Pickup overview - booking hub page

The booking detail page (`/bookings/[id]`) shows an overview hub when `booking.status === 'picked_up'` with item pickup statuses and action buttons. Consider extracting the item status list into a reusable component (or extending `BookingItemsList`) to avoid duplicating the status badge logic between the overview and the checklist.

## Merge bookings

When a user wants to consolidate two active pickups into one, there is no merge flow. Currently they must create a new booking. Deferred from pickup/return revamp.

## Personal booking deprecation

Managers cannot distinguish personal bookings (no team, no external name) from other users' personal bookings. Options: show creator name always (requires backend name join on booking response), remove personal booking creation from the UI, or deprecate the concept in favour of external-name bookings.
