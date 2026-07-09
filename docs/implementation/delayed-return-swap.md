# Delayed return - conflict handling (design plan)

Implementation plan for `docs/implementation/pre-release.md` item 11. Written before coding per the checklist's "ask first" step - this doc is the answer to "what did we decide and why," `pre-release.md` item 11 just tracks status against it.

## Problem

When an item is still `picked_up` past its booking's `end_date`, and another booking is already waiting for that exact physical unit, the waiting booking is blocked. Today nothing resolves this automatically and nothing tells the waiting booker anything is wrong.

**Scope expanded 2026-07-08** beyond the original "still out past end_date" case to cover every way a waiting booking's exact assigned unit can stop being available to it:
- The unit is still `picked_up` (delayed/overdue) - the original case above.
- The unit comes back reported broken (`reported_usable`/`reported_unusable`) or `missing` at return time - the article's own condition changes, which can affect a *different* booking that's already holding that same `article_id` for a future date.
- A booking's own dates change (via `Update`, including the copy modal's follow-up `PATCH`) and the exact item it holds isn't available for the new range - today this hard-409s; it should silently swap first.

All three reduce to the same underlying operation (find an equivalent available unit, substitute it, else fall back to a conflict/notification) via the one shared helper described in decision 3.

## Current state (as of investigation, 2026-07-08)

- `return_status` on `booking_items` already supports `delayed` (migration `00001_init.sql`). The API already requires `expected_return_date` in the request when marking an item delayed (`api/internal/handler/bookings.go:1292`) - **but never persists it**. Pure validation, then discarded. This is a real gap the whole feature depends on closing.
- `SwapBookingItemArticle` (`api/internal/handler/bookings.go:1114-1187`, `POST /bookings/{id}/items/{itemId}/swap`) is a *manual*, pickup-time, single-unit swap where the caller already knows the target article ID. It sets `pickup_status = 'swapped'`. Not reusable as-is for this feature, which needs to auto-*resolve* an equivalent unit for a booking that hasn't picked up yet.
- No `swap` event type exists in `booking_events` yet.
- The hourly `runBookingCleanupJobs` loop in `api/cmd/server/main.go` (archive expiry + archive warnings) is the natural home for a new nightly-equivalent job, per existing convention (immediate-on-startup + hourly, 1-min in dev).
- A separate *daily* scheduler (`notifications.StartScheduler`) already sends `EventBookingOverdue` to the *late* booker + managers ("your booking is overdue"). That is a different notification from what this feature needs (which notifies the *next*, blocked booker) - not to be confused or merged.
- `ReturnChecklist.svelte` already has a `delayWarning` state and a `checkConflict()` heuristic (calls the generic `checkAvailability` for the chosen date and warns if nothing of that product is available anywhere) - a rough proxy for what item 11 asks for ("show the manager the next expected user"). Plan is to extend this, not replace it.

## Decisions

1. **Persist `expected_return_date`.** New nullable column `booking_items.expected_return_date date`, set when `return_status = 'delayed'`. Left `NULL` for items the nightly job auto-detects as overdue without ever being explicitly marked (no one estimated a date for those). Migration `00020_delayed_return_swap.sql` (also adds `swap` to `booking_events_type_check`) - **written**.

2. **"Waiting booking" = any non-terminal booking holding the exact `article_id`.** `draft`, `submitted`, `approved`, `confirmed` all count. Since availability checks already prevent two active bookings from holding the same physical unit for overlapping ranges, there is normally at most one such booking per delayed article.

3. **Shared swap-resolution helper, used by every trigger below - including `Update`'s general conflict path, not just Copy.** The user shouldn't care which physical unit they're assigned, only that they have one. One operation (`FindEquivalentAvailableArticle` + swap), several callers, all part of this same commit:
   - Delayed/overdue items (mark-as-delayed + nightly job) - the original case.
   - Return-time condition changes (`reported_usable`/`reported_unusable`/`missing` - decision 6).
   - `Update`'s conflict path generally (decision 7) - covers both a normal user editing dates on an existing booking *and* the copy modal's follow-up `PATCH` (item 10), since both go through the same handler. Only surface the hard 409 if no equivalent unit exists.

6. **Condition-change triggers (`reported_usable`/`reported_unusable`/`missing`). Written.**
   - `reported_usable` (damaged but still bookable - article status stays in the available set): **opportunistic upgrade only**. If a fully-`ok` equivalent unit is available for the waiting booking, swap it in; otherwise leave the still-usable unit assigned. Never notifies - nothing is actually blocked, since a `reported_usable` article remains bookable. Implemented via a new `upgradeOnly bool` param on `ResolveBlockedItemsForArticle`, which restricts `FindReplacementArticle`'s status filter to `['ok']` instead of `['ok', 'reported_usable']` (that query's status list is now a parameter, `@statuses::text[]`, rather than hardcoded - the archive-conflict caller in `articles.go` passes the normal two-status list unchanged).
   - `reported_unusable`/`missing`: the article becomes genuinely unbookable (excluded from `AvailableArticles`'s status filter). Any waiting booking already holding that exact `article_id` needs the full swap-or-notify treatment, identical to a delayed item. Hooked into the same `reported_unusable`/`missing` branches in `UpdateBookingItemReturnStatus` that already create an issue via `POST /issues` - runs alongside that, not instead of it. `check_date` = today, since (unlike the manual mark-as-delayed flow) there's no explicit estimated date here.

7. **`Update`'s conflict path also swaps before it 409s, universally - not copy-only. Written.** Changing a booking's dates (`Update` in `api/internal/handler/bookings.go`) used to hard-conflict (`409 article_not_available_for_dates`/`articles_not_available_for_dates`) if any currently-held item wasn't available for the new range. This is the same shape of problem as the other triggers - the user doesn't care which physical unit they end up with - so `Update` now tries `FindReplacementArticle` per conflicting item first (excluding both the booking's own current items *and* any unit already handed to an earlier conflicting item in the same request, so two conflicts can't both grab the same single spare) and silently swaps via `SwapBookingItemArticleByArticle` (logging a `swap` booking_event, actor = the editing user) before falling back to the 409 for whatever's left unresolved. This is what makes "copy a booking, get new dates, it just works" true even when the exact original items are unavailable - Copy needs no special-casing, since it already goes through `Update`. Confirmed by `TestUpdateFlow_ConflictPathSwaps` and by the pre-existing `TestBookingFlow_Copy` conflict subtest now resolving its swappable item silently and 409ing only on the one with no free equivalent.

4. **Notification target for "no swap available": same pattern as archive warnings. Written.** Broadcasts to the waiting booking's team channels (email/GChat, respecting Gruppkanal prefs) *and* sends a personal email to the booking's creator only (not the full team roster - unlike `sendBookingToTeam`), deduped via `notification_log`. New `EventKey` `EventBookingItemBlocked` = `"booking_item_blocked"`, reusing the existing generic booking-email template unchanged - only new i18n keys (`notif_`/`email_subject_`/`email_banner_`/`email_intro_`/`email_cta_`, in both `sv.json`/`en.json`) plus a new `eventStyles` entry and a `BlockedItemName` field on `BookingEmailData` (set after `fetchBookingEmailData`, same pattern as `ArchiveDeadline`). The intro text interpolates `{item_name}` - added as a second var alongside the existing `{deadline}` var in both `renderBookingEmail` and `buildBookingText`'s `i18n.T` calls, harmless for every other event since unused template vars are simply ignored. `SendBookingItemBlocked` lives in `notifications/send.go`; wired from `ResolveBlockedItemsForArticle` (`handler/booking_swap.go`) as a fire-and-forget `go` call (matching every other handler-triggered notification in this codebase) whenever `FindReplacementArticle` finds nothing and `upgradeOnly` is false. Covered by `TestNotificationDispatch_BookingItemBlocked` in `internal/handler/tests/notification_dispatch_test.go`, which calls `SendBookingItemBlocked` directly (as the other dispatch tests do) rather than trying to synchronize on the async goroutine.
   - **Dedup key correction:** `HasNotificationBeenSent` keys on `(entityID, event, userID, channel)`. Every other booking notification uses the *booking's own* ID as `entityID`, which is fine because those events only fire once per booking lifecycle stage. This one can fire for *different* blocked items within the same waiting booking over time, so `entityID` must be the **`booking_item.id`**, not the booking ID, or a second blocked item in the same booking would be silently suppressed by the first one's log entry.

5. **Nightly job folds into the existing `runBookingCleanupJobs` loop** in `api/cmd/server/main.go` (hourly, 1-min in dev, runs once immediately on startup) - one less ticker to reason about, and the interval fits the "no confirmation required" fire-and-forget nature of this feature.

   **Grace period (added 2026-07-08).** The nightly job doesn't swap the instant an item is a day late - a `48h` grace period (`overdueSwapGracePeriod` in `booking_swap.go`) must pass before a never-flagged overdue item is eligible, so a booking that's merely running slightly behind isn't punished just because someone else happens to be waiting. This only applies to items nobody has flagged: if the person returning items explicitly marks one "delayed" (decision 1) the problem is already known and it always resolves immediately, with no grace period. The grace period doesn't create a "how far forward do we swap" question either - `FindWaitingBookingItemsForArticle` only ever considers a booking "waiting" once its own `start_date` has already arrived, so a booking still weeks out is never preemptively touched regardless of how overdue the blocking item is; each nightly pass only resolves whichever single booking is actually blocked *right now*.

   **Precision is day-granular, not hour-granular (decided 2026-07-09).** `graceCutoff` is computed from `time.Now()` (carries a time-of-day), but it's compared against `b.end_date`, which is date-only - so depending on what time of day the nightly job happens to run, the effective grace period enforced ranges from ~24h to ~72h rather than exactly 48h. Accepted as-is: `end_date` being date-only already means this feature doesn't reason about bookings being "a few hours overdue," only "overdue as of a given day," so hour-level precision on the cutoff wouldn't actually buy anything. Documented in code at `overdueSwapGracePeriod`.

## Design

### Trigger conditions (unified)

Four trigger points, all resolving through the same "find a waiting booking that needs this article, then find-or-notify" shape:

- **Immediate (mark-as-delayed):** `check_date` = the manager's entered `expected_return_date`. Runs synchronously inside `UpdateBookingItemReturnStatus`'s `delayed` branch, right after persisting the date.
- **Nightly job:** enumerates `booking_items` still `picked_up` where the booking's `end_date` has passed and `return_status` is `NULL` (never explicitly marked) or `delayed` (marked but not yet resolved). `check_date` = today, since there's no explicit estimate to look further ahead with.
- **Return-time condition change:** `UpdateBookingItemReturnStatus`'s `reported_usable`/`reported_unusable`/`missing` branches call the resolver synchronously for that article, `check_date` = today. `reported_usable` runs in upgrade-only mode (decision 6) - swap if a strictly-`ok` unit exists, otherwise no-op, never notify.
- **`Update`'s date-conflict path:** for each item the availability re-check would otherwise 409 on, try the swap first (decision 7), scoped to that one waiting/being-updated booking rather than a system-wide scan - this one already knows exactly which booking and which items are affected, no need to search for a "waiting booking" separately.

For the first three (delayed, nightly, condition-change), given `(article_id, check_date)`: find waiting booking_items (decision 2) whose `start_date <= check_date`, ordered by `start_date`. For each:
- Try `FindEquivalentAvailableArticle` (same `commercial_name` + `location_id`, available for the waiting booking's own date range, excluding the affected unit). If found: swap it into the waiting booking's `booking_items.article_id` (new query, does **not** touch `pickup_status` - the item hasn't been picked up yet, unlike `SwapBookingItemArticle`'s pickup-time swap), log a `swap` booking_event on the *waiting* booking (message names old/new unit; `actor_id` = the waiting booking's own `created_by`, following the same precedent as `auto_archived` events, which have no real human actor).
- If no equivalent exists: send the "no swap available" notification (decision 4, written), naming the affected item(s) but not the current (late) booker, per the doc's original privacy note. Skipped entirely for the `reported_usable` upgrade-only case.

For the fourth (`Update`'s own conflict path), the "waiting booking" is just the booking being updated itself - no search needed, just try-swap-per-conflicting-item against the new date range before deciding what's still actually conflicting.

### New sqlc queries (`api/internal/db/queries/bookings.sql`)

- `UpdateBookingItemReturnStatus` - extended to also set `expected_return_date` (cleared to `NULL` for every outcome except `delayed` - the estimate no longer applies once resolved).
- `FindWaitingBookingItemsForArticle(article_id, group_id, check_date)` - the shared trigger-condition query above; used both by the nightly scan (per delayed article found) and the immediate mark-as-delayed check, and doubles as the "next expected user" preview query for the frontend (see below) when called with `check_date` = the date currently typed into the expected-return-date field, before saving. **Written and wired for the delayed/overdue path.**
- `FindDelayedOrOverdueItems(today)` - cross-group enumeration for the nightly job (mirrors `GetAllOverdueBookings`'s cross-group shape). **Written.**
- ~~`FindEquivalentAvailableArticle`~~ / ~~`AutoSwapBookingItemArticle`~~ - **not needed**: `articles.sql` already has `FindReplacementArticle(group_id, commercial_name, location_id, exclude_ids, start_date, end_date)` and `SwapBookingItemArticleByArticle(new_article_id, old_article_id, booking_id, group_id)`, built for the archive-conflict auto-replace flow (`articles.go` bulk status-change handler) - same shape, same "doesn't touch pickup_status" behavior. Reused as-is rather than duplicating.
- `GetBlockedItemsForBooking(booking_id, group_id)` - powers the booking-detail warning section directly: this booking's own items where `start_date <= CURRENT_DATE` and another booking still holds the same `article_id` `picked_up`/unresolved. Returns holder `user_id` + `booking_id` + `expected_return_date` for the "who has it" link. **Not yet written** - part of the `Update`'s-conflict-path/notification/frontend phase, not the delayed/overdue path.

### Backend

New `api/internal/handler/booking_swap.go`, mirroring `booking_archive.go`'s shape. **Written, wired for the delayed/overdue path only:**
- `ResolveBlockedItemsForArticle(ctx, q, n, gn, baseURL, groupID, articleID, checkDate, upgradeOnly) (swapped bool, err error)` - one article, called synchronously from the mark-delayed handler (`UpdateBookingItemReturnStatus`'s `delayed`/`reported_usable`/`reported_unusable`/`missing` cases in `bookings.go`). When no equivalent unit is found (and `upgradeOnly` is false), fires `notifications.SendBookingItemBlocked` as a fire-and-forget goroutine.
- `ResolveOverdueSwaps(ctx, q, n, gn, baseURL) (swapped int, err error)` - the nightly entry point, wired into `runBookingCleanupJobs` in `main.go`, passing through the same notifiers used for archive warnings.

`Get` (booking detail, `api/internal/handler/bookings.go`) gains a `blocked_items` field in its JSON response, computed the same way `auto_approves`/`archive_deadline` already are (extra query, not stored).

New endpoint: `GET /bookings/{id}/items/{itemId}/delay-preview?expected_return_date=YYYY-MM-DD` - read-only, calls `FindWaitingBookingItemsForArticle` for that item's article and the given date, returns the blocking booking's holder info (or none). Powers the "next expected user" preview while the manager is still filling in the form, before they save.

### Notifications (`api/internal/notifications/`)

- New `EventKey` (e.g. `EventBookingItemBlocked`) + `AllEvents` entry + `BroadcastSystemDefaults` entry (team broadcast on, personal `PolicyIfNoBroadcast` - matching the archive-warning pattern).
- New i18n keys only: `email_subject_`, `email_banner_`, `email_cta_`, `email_intro_` for the event - the generic `bookingMsg`/`renderBookingEmail`/GChat opener-detail machinery is reused unchanged (confirmed by tracing `fetchBookingEmailData`/`renderBookingEmail` - fully data-driven off the event key already).
- Sender function follows `sendArchiveWarningForBooking`'s shape (broadcast to team channels + loop over `bookingRecipients`), but keyed by `booking_item.id` as `entityID` per decision 4's dedup correction, not the booking ID.

### Frontend. Written.

- `ReturnChecklist.svelte`: `checkConflict()` (triggered on `expectedReturnDate` change while marking an item delayed) now also calls `api.getDelayPreview()`. If a waiting booking is found, shows its creator via the existing compact `UserBadge` component (which already wraps `UserInfoCard`) alongside the generic "fully booked" warning - no new component needed. Also fixed: the "fully booked" text was previously hardcoded Swedish, not going through Paraglide - now `return_delay_fully_booked`/`return_delay_blocks_booking` i18n keys. For the quantity-tracked group form (which has no single item in scope), the first unhandled item in the group is used as the representative - quantity-tracked units share one `articles` row, so any one of them carries the same `article_id`.
- `bookings/[id]/+page.svelte`: new warning section (visible once `blocked_items` is non-empty) listing each blocked item, reusing `UserBadge` (not a bare `UserInfoCard` wire-up - `UserBadge` already is the "compact badge that opens the full popup on click" component per item 6) with `contextBookingId` = the holder's booking.
- `client.ts`: `BlockedItem`/`DelayPreview` types, `blocked_items` added to `getBooking`'s response type, new `getDelayPreview()` method.
- Backend support for the above: `GetBlockedItemsForBooking` query, `blocked_items` field on `Get`'s JSON response, and the new `GET /bookings/{id}/items/{itemId}/delay-preview` endpoint (`DelayPreview` handler in `bookings.go`) - documented in `docs/API.md`. Covered by `TestDelayPreview` and `TestGetBooking_BlockedItems` in `booking_swap_test.go`.

## Known simplifications / accepted limitations

- Swap resolution only ever considers the *exact same* `commercial_name` + `location_id` - no cross-location substitution, matching how every other availability query in the codebase already groups articles.
- If more than one waiting booking somehow holds the same affected article (shouldn't happen given availability checks, but not schema-enforced), only the earliest by `start_date` is resolved per pass; the next nightly tick will catch any that remain unresolved for the delayed/overdue/condition-change triggers. `Update`'s own conflict path doesn't have this ambiguity - it only ever concerns the one booking being edited.
- Item 10's copy flow needs **no special-casing at all** now that `Update`'s conflict path swaps universally (decision 7) - `Copy`'s follow-up `PATCH` already goes through `Update`, so it gets this for free.
