# Backlog

Deferred work items. These are things we've identified as needed but are withholding for now.

## Date change conflict UX

When changing dates on a booking, the API validates that all existing items are available for the new range and returns 409 with the conflicting article name. Currently the UI just shows a red error. Better UX: highlight which items conflict and let the user remove them before retrying the date change.

## Copy booking (deferred to packages/kits)

Copying a booking needs availability checking against the new dates. This ties into the package/kit feature (Phase 3) since both involve populating a booking from a template. The API endpoint (`POST /api/v0/bookings/{id}/copy`) exists but the UI is deferred.

## Booking editing — change tracking via booking_events

The `booking_events` table now supports mutation tracking event types (`items_changed`, `dates_changed`, `details_changed`) with a `metadata` jsonb column for structured diffs. This enables:

- **Change history**: log what changed when items are added/removed, dates are moved, or unit/notes are updated. The metadata captures old/new values.
- **Undo/restore**: with a full change history, we can implement "revert to previous state" by replaying events backwards.
- **Audit trail**: managers can see exactly what the leader changed between rejection and resubmission.

Currently only approval-flow events (`submitted`, `approved`, `rejected`) and `note` events are logged. Mutation events are the next step.

**Implementation plan:**
1. Log `items_changed` events in AddItems/RemoveItem handlers with metadata like `{"added": ["Sibley 1"], "removed": []}` or `{"added": [], "removed": ["Stormkök 3"]}`
2. Log `dates_changed` events in Update handler with `{"old_start": "2026-06-01", "new_start": "2026-06-05", ...}`
3. Log `details_changed` for unit/notes changes
4. Show these in the booking event thread alongside approval messages
5. Eventually: "revert this change" button that undoes a specific mutation

## Draft auto-cleanup

Draft bookings with zero items that are abandoned are automatically deleted after 48 hours. A background goroutine in the Go API runs hourly and calls `CleanupEmptyDrafts`. For drafts with items (stale but not empty), cleanup is deferred until notifications are implemented — users should be notified before their booking is deleted.

**Status: partially resolved** — empty draft cleanup implemented. Stale drafts with items deferred pending notification system.

## Quantity-tracked items — manager UI

After CSV import, the equipment manager needs a UI to:
- Toggle an article group between individually tracked and quantity tracked
- Adjust the count for quantity-tracked items (add/remove records)
- View and edit tracking mode per article group

**Status: deferred** — waiting for proper admin/management UI.

## Subcategories

Categories are flat. Items like tents (Sibley, Primus) are under "Sova" but would benefit from a subcategory "Tält". The `categories` table already supports `parent_id` but the UI doesn't use it yet.

## Date picker locale

The native `<input type="date">` uses the browser/OS locale for week start day and time format. Some users see Sunday-Saturday instead of Monday-Sunday. Full control requires a custom date picker component.

## i18n system

All user-facing strings are currently hardcoded in Swedish. Need to set up an i18n system (e.g. paraglide-sveltekit) before adding English as a second language.

## Unavailable items in copied bookings

When copying a booking, items that exist but are no longer available for the new dates should be visually marked (not silently included). Currently they're included without availability checking since the copy has placeholder dates.

## Race conditions on concurrent edits

Two users editing the same booking simultaneously could cause conflicts. No optimistic locking or conflict detection exists yet. The booking detail page now polls every 10s during active statuses (`draft`, `submitted`, `confirmed`, `picked_up`) so concurrent users see each other's progress. For a scout group with 2-3 leaders this is sufficient. Consider adding `updated_at` checks on write operations if conflicts become a real problem.

## Project OIDC claims — investigation needed for Phase 3

Projects (e.g. "Valborg 2026", "Scoutläger") are stored in the `units` table with `type = 'project'`. Project membership comes from OIDC token claims, same as unit membership. Before implementing real auth (Phase 3 Step 1), we need to:

1. Inspect a real ScoutID token for a user with project roles (e.g. "Lägeransvarig", "Valborgsansvarig")
2. Determine how projects are represented — same claim as units? Separate claim? Prefixed?
3. Design the claim-to-project mapping in the Go API JWT validation
4. Decide if projects need additional metadata (start/end dates, description, active/archived status)

Currently in dev mode, project names are in `claims.Units` alongside unit names. This may need to change if the OIDC token separates them.

## Pickup — report missing items

When confirming pickup of quantity-tracked items with a count lower than booked, prompt the user to report the missing items. This could auto-create an issue report for the equipment manager. Currently the shortfall is silently marked as `not_available`.

## Manager article swap on active bookings

Equipment managers need the ability to swap out which specific article is assigned to a booking, even after confirmation. Use cases:
- An article becomes unexpectedly unavailable (broken between bookings, lent informally, etc.)
- A delayed return from another booking blocks an assigned article
- Rebalancing inventory across locations

The swap endpoint exists for pickup (`POST /bookings/{id}/items/{itemId}/swap`) but is restricted to `picked_up` status. Managers should be able to swap on `confirmed`/`approved` bookings too.

## Delayed return — conflict resolution

When an article is returned late (`delayed` status) and overlaps with another booking that has the same article assigned:
- Option A: auto-swap the article in the affected booking for a free equivalent (transparent to the other booker)
- Option B: notify the equipment manager to manually resolve
- Option C: both — auto-swap if possible, alert manager if no equivalent available

Currently the system shows a warning when picking a delayed return date that conflicts, but doesn't resolve the conflict.

## Date validation and overdue handling

Several date-related edge cases need attention:
- **Overdue bookings**: bookings with `end_date` in the past that are still `picked_up` — should show a visual warning, possibly notify the manager
- **Delayed return date in the past**: the UI currently allows entering an `expected_return_date` that's already passed — should validate or warn
- **Booking start date in the past**: creating or editing bookings with past dates — should this be allowed? Managers might need it for retroactive bookings
- **Overdue reminder schedule**: periodic notifications for unreturned items (daily? configurable?)

## Quantity-tracked items — return flow

Quantity-tracked items (e.g. 5× Tältlampa LED) need a grouped return UI similar to the pickup flow:
- Show one row per product group with a number input for how many are returned OK
- Allow marking some as broken/lost/delayed with a count
- Currently the return checklist shows individual rows for quantity-tracked items, which is confusing since they all have the same name

## Issue reporting — booking context in event history

Article event history should clearly indicate whether an issue was reported:
- Before a borrow (pre-existing issue discovered during browse/inventory)
- During a borrow (reported as part of pickup or return flow)
- As a standalone report (not related to any active booking)

This could be done by linking article events to booking IDs when they originate from a booking flow, and showing that context in the history UI (e.g. “Rapporterad vid återlämning av bokning #X”).

## Ärenden — role-scoped visibility

The Ärenden (issues) page is now visible to all users with role-appropriate controls:
- **Equipment manager** — sees all articles with issue/repair/lost status, can change status
- **Leader / project leader** — sees the same article list but read-only (no status change controls), filter options exclude manager-only statuses (under_repair, archived)

Future improvement: filter to only show articles the user personally reported, or articles reported by members of their unit. This requires a new query parameter (e.g. `reported_by=me`) that filters articles to those with an `issue_reported` event by the current user.

**Status: partially resolved** — role-based UI implemented with leader status transitions, per-user filtering deferred.

## Article event history — limit display

The article event history endpoint now supports a `?limit=N` parameter. The frontend loads the 10 most recent events by default and shows a "Visa alla" button when more exist.

**Status: resolved.**

## CSV import — quantity-tracked items

The CSV import supports a `count` column. Rows with `count > 1` create multiple quantity-tracked articles (`individually_tracked = false`) from a single row. Rows without a count (or count=1) are individually tracked. Column order doesn't matter — all columns are resolved by header name.

**Status: resolved.**

## Pickup state management

Several improvements needed for the pickup flow:

### Undo all pickups → revert to confirmed/approved
When all items in a picked_up booking have their pickup status cleared (undone), the booking automatically transitions back to its pre-pickup status (confirmed or approved). The `pre_pickup_status` column on the `bookings` table stores the status before pickup.

**Status: resolved.**

### Partial pickup indication
When some but not all items are picked up, the booking should visually indicate "partial pickup" status. A "Klar med uthämtning" (done with pickup) button should let the user confirm they're finished even if not all items were picked up. Items left unmarked would be treated as not collected.

### Adding items after full pickup
If new items are added to a booking that was fully picked up, the status should revert to partial pickup since the new items haven't been collected yet.

## Pickup date validation

The API currently allows transitioning a booking to `picked_up` regardless of the booking's start date. Pickup should only be allowed on or after the booking's start date — the booking dates represent the full period you have the gear, including any prep days. Picking up before the start date would mean unaccounted-for gear outside the booked window, breaking availability for others. Picking up *after* the start date is normal (book Wednesday for prep flexibility, actually pick up Friday).

The browse/inventory view should distinguish between "reserved for today but not yet picked up" and "currently checked out" — both matter when you're physically at the storage and need to know what's spoken for.

## Browse page — date picker for time-travel view

The browse page shows inventory state for today by default (`with_availability=true&date=today`). Add a date picker that lets users view the state at any date — "what was checked out last Tuesday?" or "what's reserved for next Friday?". The API already supports a `date` param on the availability-enriched article list.

The booking page should also use this: when viewing a booking for June 15-20, the availability view should show what's available for *those* dates, not today.

## Seed script — date sprawl for realistic history

Article events created by the seed script all have `created_at = now()`, making the history timeline unrealistic. After creating events through the API, the seed script should backdate them via direct SQL (`docker compose exec db psql -c "UPDATE article_events SET created_at = ... WHERE ..."`). This gives a realistic spread: issue reported 2 weeks ago, manager set under_repair 10 days ago, resolved 3 days ago, etc.

No API changes needed — this is purely a seed script improvement for dev/demo purposes.

## User instructions / help page

Add a user-facing instruction page ("Så här fungerar det" / "Hjälp") accessible from the UI navigation. Content lives in a markdown file (`docs/user-guide.md`) but renders nicely in the web app. Should cover:
- How to browse and book equipment
- The approval flow (when is approval needed, what happens after submit)
- Pickup and return checklists
- How to report issues
- Role differences (what leaders vs equipment managers see)

For demo mode, this doubles as the demo walkthrough — explaining what to try and how the persona switcher works.

## Booking flow — date change with items in cart

When a booking already has items in the cart and the user changes the dates, the existing items are not re-validated against the new date range. This can lead to double-bookings or items that aren't actually available for the new dates. Currently the dates are editable after items are added, which breaks the flow.

Temporary fix: disable date editing after items have been added to the booking. Proper fix: re-validate all items when dates change (the API already does this on `PUT /bookings/{id}` with date changes, returning 409 for conflicts — the UI needs to handle this gracefully, showing which items conflict and letting the user remove them).

