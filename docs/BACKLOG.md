# Backlog

Deferred work items. These are things we've identified as needed but are withholding for now.

## Date change conflict UX

When changing dates on a booking, the API validates that all existing items are available for the new range and returns 409 with the conflicting article name. Currently the UI just shows a red error. Better UX: highlight which items conflict and let the user remove them before retrying the date change.

## Copy booking (deferred to packages/kits)

Copying a booking needs availability checking against the new dates. This ties into the package/kit feature (Phase 3) since both involve populating a booking from a template. The API endpoint (`POST /api/v0/bookings/{id}/copy`) exists but the UI is deferred.

## Booking editing — undo/restore

When editing a confirmed booking, it currently transitions to draft. There's no way to undo changes and restore the previous state. Options to explore:
- Copy-on-edit: snapshot the booking before editing, restore on cancel
- Versioned bookings: track changes as a history, allow rollback
- For now: editing puts booking in draft, user must re-submit. Abandoned drafts cleaned up automatically.

## Draft auto-cleanup

Draft bookings that are abandoned should be automatically deleted after a configurable period (e.g. 24 hours). The `CleanupStaleDrafts` query exists but is not yet called from anywhere. Options:
- Cron job / scheduled task in the Go API
- Cleanup on startup + periodic goroutine
- External scheduler (e.g. pg_cron)

## Quantity-tracked items — manager UI

After CSV import, quantity-tracked items (e.g. LED tent lights) have only 1 record. The equipment manager needs a UI to:
- Mark an item as quantity-tracked (`individually_tracked = false`)
- Set the actual count (creates additional records)

## Subcategories

Categories are flat. Items like tents (Sibley, Primus) are under "Sova" but would benefit from a subcategory "Tält". The `categories` table already supports `parent_id` but the UI doesn't use it yet.

## Date picker locale

The native `<input type="date">` uses the browser/OS locale for week start day and time format. Some users see Sunday-Saturday instead of Monday-Sunday. Full control requires a custom date picker component.

## i18n system

All user-facing strings are currently hardcoded in Swedish. Need to set up an i18n system (e.g. paraglide-sveltekit) before adding English as a second language.

## Unavailable items in copied bookings

When copying a booking, items that exist but are no longer available for the new dates should be visually marked (not silently included). Currently they're included without availability checking since the copy has placeholder dates.

## Race conditions on concurrent edits

Two users editing the same booking simultaneously could cause conflicts. No optimistic locking or conflict detection exists yet. Consider adding `updated_at` checks on write operations.

## Pickup — report missing items

When confirming pickup of quantity-tracked items with a count lower than booked, prompt the user to report the missing items. This could auto-create an issue report for the equipment manager. Currently the shortfall is silently marked as `not_available`.
