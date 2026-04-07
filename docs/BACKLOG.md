# Backlog

Deferred work items — things to grab when there's time, smaller tasks set aside during major work. When an item is completed, move it to [accomplished.md](accomplished.md).

## Admin UI for group/role management

Currently, group access is controlled by a static `role-mapping.json` file. Users from groups not in this file see a "group not configured" message. Future plan:

- Auto-register groups/troops/roles when users attempt to log in (store what was seen in the token)
- Admin interface where managers can:
  - See all groups that have attempted login
  - Grant groups access to the system
  - Add groups by Scoutnet role or group ID
  - Assign access levels per unit/role: none, normal (booking access), low (project leader level), manager
- Replace "project" concept with "funktionärsroll" (functional role) — a role should be given none, normal, low, or manager level access
- A unit should be given normal access to the booking system by default

This replaces the current static role-mapping approach and enables self-service onboarding for new scout groups.

## Date change conflict UX

When changing dates on a booking, the API validates that all existing items are available for the new range and returns 409 with the conflicting article name. Currently the UI just shows a red error. Better UX: highlight which items conflict and let the user remove them before retrying the date change.

## Copy booking

Copying a booking needs availability checking against the new dates. Ties into the package/kit feature (Phase 4) since both involve populating a booking from a template. The API endpoint (`POST /api/v0/bookings/{id}/copy`) exists but the UI is deferred.

## Booking editing — change tracking via booking_events

The `booking_events` table supports mutation tracking event types (`items_changed`, `dates_changed`, `details_changed`) with a `metadata` jsonb column. Currently only approval-flow events and `note` events are logged. Next steps:

1. Log `items_changed` events in AddItems/RemoveItem handlers
2. Log `dates_changed` events in Update handler
3. Log `details_changed` for unit/notes changes
4. Show these in the booking event thread
5. Eventually: "revert this change" button

## Stale drafts with items

Empty draft cleanup is implemented (48h). Stale drafts *with* items need notifications before deletion — users should be warned before their booking is removed.

## Quantity-tracked items — manager UI

Equipment manager needs UI to:
- Toggle article groups between individually tracked and quantity tracked
- Adjust count for quantity-tracked items
- View and edit tracking mode per article group

## Subcategories

Categories are flat. The `categories` table supports `parent_id` but the UI doesn't use it yet.

## Date picker locale

Native `<input type="date">` uses browser/OS locale for week start day. Full control requires a custom date picker component.

## i18n system

All user-facing strings are hardcoded in Swedish. Need to set up an i18n system (e.g. paraglide-sveltekit) before adding English.

## Unavailable items in copied bookings

When copying a booking, items that aren't available for the new dates should be visually marked, not silently included.

## Race conditions on concurrent edits

No optimistic locking. Booking detail polls every 10s during active statuses. Consider `updated_at` checks on writes if conflicts become a real problem.

## Pickup — report missing items

When confirming pickup of quantity-tracked items with a count lower than booked, prompt the user to report missing items. Currently the shortfall is silently marked as `not_available`.

## Manager article swap on active bookings

Managers should be able to swap articles on `confirmed`/`approved` bookings, not just during pickup (`picked_up` status). Use cases: unexpected unavailability, delayed returns from other bookings, inventory rebalancing. When an article is given a new status, managers should be informed about potential unavailability issues.

## Delayed return — conflict resolution

When a delayed article overlaps with another booking:
- Auto-swap for a free equivalent if possible
- Alert manager if no equivalent available

Currently shows a warning but doesn't resolve the conflict.

## Date validation and overdue handling

- Overdue bookings (`end_date` in past, still `picked_up`) — visual warning, manager notification
- Delayed return date in the past — validate or warn
- Booking start date in the past — allow for retroactive bookings?
- Overdue reminder schedule (daily? configurable?)

## Quantity-tracked items — return flow

Grouped return UI for quantity-tracked items: one row per product group with count inputs instead of individual rows with identical names.

## Issue reporting — booking context in event history

Link article events to booking IDs when they originate from a booking flow. Show context like "Rapporterad vid återlämning av bokning #X".

## Ärenden — per-user filtering

Filter issues page to only show articles the user personally reported. Requires a query parameter (e.g. `reported_by=me`) filtering on article events.

## Pickup state — partial indication

When some but not all items are picked up, visually indicate partial pickup. A "done with pickup" button to confirm even if not all items were collected.

## Pickup state — adding items after full pickup

If new items are added to a fully picked-up booking, revert to partial pickup state.

## Pickup date validation

Only allow pickup on or after the booking's start date. Browse/inventory should distinguish "reserved for today but not yet picked up" from "currently checked out".

## Browse page — date picker for time-travel view

Add a date picker to view inventory state at any date. The API already supports a `date` param.

## Seed script — date sprawl

Backdate article events in the seed script for realistic history spread. No API changes needed.

## Booking flow — date change with items in cart

When dates change on a booking with items, re-validate all items against the new range. Currently the API returns 409 for conflicts but the UI doesn't handle it gracefully. Temporary fix: disable date editing after items are added.
