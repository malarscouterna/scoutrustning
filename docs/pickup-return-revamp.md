# Pickup / Return Flow Revamp

## Summary

Fix several UX and state-management problems in the pickup and return flow. No new tables or API endpoints needed - changes are primarily frontend with one backend fix.

---

## Problems being solved

### User-reported

1. **Quantity-tracked groups don't distinguish ok vs reported-usable items** - all items shown as one undifferentiated group.
2. **Partial pickup state is invisible** - picking up 2 of 3 items shows nothing until all 3 are done.
3. **Ångra reverts booking to confirmed** - undoing all pickups calls the API which reverts the booking status, causing the polling to navigate away from the checklist and producing "booking must be in picked_up status" errors on retry.
4. **Redigera on a picked_up booking causes bad state** - editing reverts to draft and triggers re-approval even for unchanged items.
5. **User is forced out of the pickup/return view by polling** - when booking status changes (e.g., reverts to confirmed), the poll update causes the view to switch.
6. **Personal booking label misleading for managers** - managers see "Personlig bokning" instead of who made it.

### Found during code review

7. **`markQuantityGroup` stale-items bug** — when `extraNeeded > 0`, it calls `reload()` then immediately filters from the `items` prop. Svelte hasn't flushed the parent's reactive update yet so `updatedItems` is the old list and items get the wrong `pickup_status` set.
8. **`loadExtraAvailability` fires on every item update** — in a `$effect` that depends on `quantityGroups`, which changes on every `reload()`. During pickup this fires 1 extra API call per quantity group on every single action.
9. **`activeItemId`/`activeGroupKey` not cleared on external updates** — if polling updates `items` while a return form is open, the form stays attached to a stale item. Submitting it can apply the wrong status.
10. **"Slutför återlämning" has no loading guard** — double-tapping sends two `returnBooking` requests; the second returns 400 and shows as an error.
11. **`confirmGroupForm` stale count input** — if a user opens a group form, cancels, then reopens it, `quantityInputs[g.key_form]` may carry the previous session's value.
12. **`reopenBooking` diverges local state from API** — sets `booking.status = 'picked_up'` locally without an API call. If the user navigates away before making a return action, the page re-shows `returned` state.
13. **Poll races with `reload()`** — `PickupChecklist.reload()` and the page-level poll both call `api.getBooking` and both call `onUpdate(items)`. They can interleave, causing a stale response to overwrite a fresh one.

---

## Changes

### 1. Quantity-tracked pickup rows - split by status

**Current behaviour:** One row per `commercial_name + location_name` group regardless of item statuses.

**New behaviour:** One row per `commercial_name + location_name + status_category`, where status categories are:

| Category | Statuses included | Row colour |
|---|---|---|
| `ok` | ok, incoming, under_repair | White/default |
| `reported_usable` | reported_usable | Amber / orange-50 |
| `reported_unusable` | reported_unusable, reported_missing | Red / red-50 |

Rules:
- If all items in a group are `ok`, show one row (current behaviour).
- If the group has a mix, show multiple rows: one for ok items, one for reported_usable, one for reported_unusable. Each row shows its own count: "2 st (OK)", "1 st (Felrapporterad - kan användas)".
- `reported_unusable` rows are shown greyed/disabled with a "Ej tillgänglig" label - cannot be picked up, no pickup button.
- `reported_usable` rows are amber. They are expandable and when expanded show the active issue title + description for those items. Include a pickup button ("Hämtad ändå") and a "Ta bort från bokning" option.
- `ok` rows work as before: count picker + Bekräfta.

**"Ta bort från bokning"** removes the items from the booking (calls existing `removeBookingItem` per item in the sub-group). Only shown for non-picked-up items. `DELETE /bookings/{id}/items/{itemId}` already permits `picked_up` booking status via `isEditable()` — no backend change needed here.

**Expand content for reported_usable rows:** Show the first open issue linked to an article in the group:
- Issue title
- Issue description
- Severity badge

Fetch this from the existing `GET /api/v0/issues?article_id=...` or from the `article_status` + issue data already on the booking item. Since booking items don't currently carry issue details, load them on expand via `GET /api/v0/issues?article_id={article_id}&status=open` for one representative article in the group.

---

### 2. Partial pickup state visible mid-flow

**Current behaviour:** A quantity group row shows the confirmed state ("2 / 3 st hämtade" + Ångra) only when `groupIsDone` is true (all items have a pickup_status).

**New behaviour:** Show the confirmed state ("X / N st hämtade") as soon as any item in the group has a pickup_status. The picker and Bekräfta button remain visible so the user can still adjust the count. The Ångra button reverts all pickups in the group.

This lets a user naturally say "I only needed 2 of the 3 booked" - they set the count to 2, confirm, and see "2 / 3 st hämtade". The third item stays with no pickup_status and can be left for someone else or removed via "Ta bort från bokning".

For individually tracked items: no change needed (each item already has its own row with its own state).

---

### 3. Remove auto-revert to confirmed on ångra (backend)

**File:** `api/internal/handler/bookings.go` - `UpdateItemPickup`

**Current behaviour:** When all items are undone (pickup_status cleared), the handler calls `NoItemsPickedUp` and if true reverts the booking from `picked_up` to its `pre_pickup_status` (confirmed/approved).

**New behaviour:** Remove this auto-revert logic entirely. Once a booking enters `picked_up` status it stays there until:
- The user explicitly cancels the booking, or
- All items are returned (transitions to `returned`).

This eliminates the "booking must be in picked_up status" error on retry and the polling-induced view navigation.

The `pre_pickup_status` column and `GetPrePickupStatus`/`SetPrePickupStatus` queries can be kept for now (used by no other path) but are no longer written/read in `UpdateItemPickup`.

---

### 4. Block booking-level editing once pickup has started (frontend + backend)

**Frontend:** Hide the "Redigera" button when `booking.status === 'picked_up'`. Instead show a hint:

```
Vill du lägga till mer utrustning? Skapa en ny bokning.
```

The hint links to `/browse`.

**Backend:** In `PUT /bookings/{id}` (the booking metadata update endpoint - dates, notes, team - used by the cart/book page), add `picked_up` to the status check that returns 400. The handler currently uses `isEditable()` which already includes `picked_up`; change that check to exclude `picked_up` for this endpoint only. The `isEditable()` helper is also used by `RemoveItem` (DELETE item) and `AddItem` (POST item) — those should continue to allow `picked_up` since item-level changes during pickup are intentional.

This avoids the approval re-trigger issue entirely: there is no full-booking edit path once pickup begins.

**Backlog:** Add "Merge bookings" as a future feature for when users want to consolidate two active bookings.

---

### 5. Stay in pickup/return view during polling

**File:** `web/src/routes/bookings/[id]/+page.svelte`

**Current behaviour:** The poll updates `booking.status`. If status changes (e.g., due to the now-removed auto-revert), the `{#if booking.status === 'picked_up'}` block re-evaluates and can switch the displayed view.

**New behaviour:** With §3 implemented (no auto-revert), this problem largely goes away. However, add a `pickupStarted` local flag that is set to `true` when `startPickup()` is called and never reset by polling. Use this to keep the PickupChecklist visible even if a poll returns an unexpected status:

```svelte
let pickupStarted = $state(false);

// In startPickup():
pickupStarted = true;

// In the template:
{#if booking.status === 'picked_up' || pickupStarted}
  {#if showReturn} ... ReturnChecklist ... {:else} ... PickupChecklist ... {/if}
{/if}
```

Also: once `showReturn = true`, never set it back to false based on polling - only the user can exit return mode.

---

### 6. Remove personal booking label from UI

**File:** `web/src/routes/bookings/[id]/+page.svelte`

Remove the `{:else}Personlig bokning{/if}` branch. When a booking has no team and no external name, show nothing. The booking response contains `created_by` (user ID only, no name join), so showing creator name would require a backend change — not worth it for now.

**Backlog:** Personal bookings - decide whether to support them long-term or remove the concept. Currently managers cannot distinguish their own personal bookings from other users' personal bookings. Options: show creator name always, remove the personal booking creation option from the UI, or deprecate the concept in favour of external-name bookings.

---

### 7. Fix `markQuantityGroup` stale-items bug

After `reload()` completes, use the return value directly rather than re-filtering from the `items` prop:

```ts
const freshItems = await reload(); // reload returns BookingItem[]
const updatedItems = freshItems.filter(...);
```

`reload()` already calls `onUpdate` so the parent will update, but for the immediate loop we use `freshItems` directly.

### 8. Remove `loadExtraAvailability` entirely (superseded by Proposal A)

**Proposal A** removes the `max` cap on the count picker entirely, so `extraAvailable` and `loadExtraAvailability` become unused. Delete both. Do not implement the "load once on mount" approach — just remove the feature. The API rejects if the count exceeds what is available.

### 9. Clear active form on external item update

When `items` updates externally (from poll or `onUpdate`), if the `activeItemId` or `activeGroupKey` no longer corresponds to an unhandled item, clear it:

```ts
$effect(() => {
  if (activeItemId && !items.find(i => i.id === activeItemId && !i.return_status)) activeItemId = null;
  if (activeGroupKey) { /* similar check */ activeGroupKey = null; }
});
```

### 10. Guard "Slutför återlämning" against double-tap

Add a `completing = $state(false)` flag, set it on click, disable the button while true:

```svelte
<button disabled={!canComplete || completing} onclick={async () => { completing = true; try { ... } finally { completing = false; } }}>
```

### 11. Reset `quantityInputs[g.key_form]` on form open

In `openGroupForm`, always reset the count input to the current unhandled count rather than reading the potentially stale stored value:

```ts
quantityInputs[`${g.key}_form`] = unhandled.length;
```

### 12. Fix `reopenBooking` local state

The current `reopenBooking` correctly sets `booking = { ...booking, status: 'picked_up' }` locally (needed so the template renders the return checklist). What is wrong is that it does not call any API — the server-side reopen happens lazily on the first `updateItemReturn` call. This means if the user navigates away before making a return action, the page re-fetches `returned` status.

Fix: keep the local state change (`booking.status = 'picked_up'`). Additionally call a no-op that triggers the server reopen — the simplest is `api.updateItemReturn` with `return_status: ''` on any already-returned item, which the `UpdateItemReturn` handler accepts and uses to reopen the booking. Alternatively, if no clean item is available, accept the lazy-reopen behaviour as a known minor issue and leave it for now (the next return action will fix it). This is low priority.

### 13. Eliminate poll/reload race

Give the page-level poll and `PickupChecklist.reload()` a shared "in-flight" guard. Simplest approach: lift `reload` out of the checklist components into the parent page and pass it as a prop. The parent owns one `loading` flag; while a reload is in flight, the poll skips its update:

```ts
let reloading = false;
async function reload() {
  reloading = true;
  try { const r = await api.getBooking(...); items = r.items; } finally { reloading = false; }
}
// In poll:
if (!reloading) { /* update */ }
```

---

## Simplification proposals (needs decision)

The following are optional simplifications that would reduce bug surface significantly. Each has a trade-off. **Needs explicit acceptance before implementing.**

---

### A. Simplify inline extra-item path; add sheet for new article types (accepted)

**Two paths for adding items during pickup:**

**1. More of an existing article (inline - keep, simplify):**

The count picker in `markQuantityGroup` can still go above the booked count. But remove `loadExtraAvailability` and the `max` attribute entirely - just let the user enter any number and confirm. If the API rejects (not available, approval required), show the error inline. No pre-fetch, no `extraAvailable` state, no "max N" label.

Fix Issue 7 (stale items) by using the return value from `reload()` directly instead of re-filtering from the prop:

```ts
const freshItems = await reload();
const updatedItems = freshItems.filter(...);
```

This eliminates Issue 8 (excessive API calls) and Issue 7 in one go, while keeping the inline UX.

**2. A new article type not in the booking (sheet):**

A "Lägg till utrustning" button opens `AddItemSheet.svelte`. Same pattern as `ReportIssueSheet`. Article search, +/- quantity picker. On confirm calls `addBookingItems` - item appears in checklist as unpicked. No approval pre-filtering on the frontend: let the API reject if approval is required, and show the error in the sheet.

Available during `picked_up` status (both pickup and return phases).

**New files:** `web/src/lib/components/AddItemSheet.svelte`

**Backend:** No new endpoints. The existing `POST /bookings/{id}/items` already enforces availability and approval server-side.

**`addBookingItems` API signature:** `addBookingItems(bookingId, commercialName, quantity, locationName?)` — takes a commercial name + optional location, not an article ID. `AddItemSheet` should group search results by `commercial_name + location_name` and pass those values on confirm. If the user selects a specific article from search, use its `commercial_name` and `location_name` as the call parameters.

---

### B. Remove `pre_pickup_status` column (accepted)

**Current:** `pre_pickup_status` on `bookings` stores the previous status (confirmed/approved) so it can be restored when all pickups are undone. With Issue 3 fixed (no auto-revert), this column is never written to or read during normal flow.

**Decision:** Drop the column entirely. Remove the three sqlc queries and their call sites. Add a migration to drop the column.

**Status history is not lost:** `booking_events` already logs every status transition with actor and timestamp. The `pre_pickup_status` value is redundant with that log.

**Action:**
- Add goose migration: `ALTER TABLE bookings DROP COLUMN pre_pickup_status;`
- Remove `SetPrePickupStatus` call from `Pickup` handler
- Remove `GetPrePickupStatus`/`NoItemsPickedUp` calls from `UpdateItemPickup`
- Delete those three queries from `api/internal/db/queries/bookings.sql` and run `sqlc generate`

---

### C. Lift reload fully into the parent page (accepted)

**Current:** Both `PickupChecklist` and `ReturnChecklist` contain their own `reload()` function that calls `api.getBooking` and then calls `onUpdate`. The parent page also polls independently. This creates the race condition in Issue 13.

**Proposed simplification:** Remove `reload()` from both checklist components. Replace every internal `await reload()` call with `await onUpdate()` - but make `onUpdate` async and have the parent do the fetch. Parent owns one `reloading` flag. Poll skips if `reloading`.

This makes the child components purely display: they receive `items`, call callbacks, never fetch independently.

**Trade-off:** More props threading. But the child components become much simpler and testable.

---

### D. Remove swap during pickup for quantity-tracked items (consider)

**Current:** `PickupChecklist` has a "Felanmäl" button on quantity groups that opens `ReportIssueSheet`. After reporting, `onReported` tries to trigger a swap for individually-tracked items. For quantity-tracked items `pendingSwapArticleId` is null so no swap is triggered - only the issue is created.

The swap flow (`startSwap`, `confirmSwap`, `swapCandidates`) applies only to individually-tracked items. For quantity-tracked items the flow is: Felanmäl - the issue gets created - the item is now `reported_unusable` - the status sub-row system (Issue 1) shows it as non-pickable.

**Observation:** The swap flow for individually tracked items is well-defined and useful (pick a specific replacement). For quantity-tracked groups, the "swap" concept doesn't apply cleanly - you'd just be picking a different unit of the same type which already happens via the count picker.

**Proposal:** No change needed here - the swap flow is correctly gated to individually tracked items only. Documenting this to confirm there is no hidden complexity to remove.

---

### E. Remove the "Starta återlämning" button gating on `anyPickedUp` (accepted)

**Current:** The "Starta återlämning" button only appears once at least one item has `pickup_status !== null`. If a user picked up 3 items, undoes all 3, the button disappears.

**Problem:** With Issue 3 fixed (no auto-revert), the booking stays in `picked_up` even with zero items picked up. A user who undoes everything and then wants to start a return would have no path forward except picking something up first.

**Proposed simplification:** Always show "Starta återlämning" when `booking.status === 'picked_up'`, regardless of `anyPickedUp`. The return checklist already handles the case where items have no `pickup_status` (shows them as "Ej hämtad"). No functional harm in starting a return with zero items picked up - the "Slutför" button only enables when all picked items are handled.

**Trade-off:** None visible. Simpler logic, fewer edge cases.

---

## Implementation order

1. **Backend** - Remove auto-revert in `UpdateItemPickup` (`bookings.go`). (Issue 3)
2. **Backend** - Block PUT booking edit if status is `picked_up`. (Issue 4)
3. **Frontend `+page.svelte`** - Lift `reload` into the parent, add poll/reload race guard (Issue 13). Hide Redigera for `picked_up`, add hint (Issue 4). Add `pickupStarted` guard (Issue 5). Remove personal booking label (Issue 6). Fix `reopenBooking` (Issue 12).
4. **Frontend `PickupChecklist.svelte`** - Split quantity groups by status category, show partial pickup state, add "Ta bort från bokning" option, add issue-detail expand for reported_usable rows (Issues 1, 2). Simplify `markQuantityGroup`: remove `loadExtraAvailability`, remove `max` cap, use reload return value directly (Issues 7, 8, Proposal A). Add "Lägg till utrustning" button → `AddItemSheet`.
5. **Frontend `ReturnChecklist.svelte`** - Clear active form on external update (Issue 9). Guard Slutför button (Issue 10). Reset count input on form open (Issue 11). Add "Lägg till utrustning" button → `AddItemSheet`.
6. **Frontend `AddItemSheet.svelte`** - New component: article search, +/- quantity picker, availability + approval filtering, calls `addBookingItems` on confirm (Proposal A).

---

## Files changed

| File | Change |
|---|---|
| `api/internal/handler/bookings.go` | Remove auto-revert in `UpdateItemPickup`; block `PUT /bookings/{id}` edit if `picked_up` |
| `web/src/routes/bookings/[id]/+page.svelte` | Hide Redigera for `picked_up`, add new-booking hint, `pickupStarted` guard, remove personal booking label |
| `web/src/lib/components/PickupChecklist.svelte` | Split quantity groups by status, partial pickup visibility, reported_usable expand with issue info, remove-from-booking option; simplify count picker (cap at booked count, remove extra-availability fetch) |
| `web/src/lib/components/ReturnChecklist.svelte` | Clear active form on external update, guard Slutför button, reset count input on form open |
| `web/src/lib/components/AddItemSheet.svelte` | New: slide-up sheet for adding items mid-pickup/return, article search with +/- quantity, availability-filtered results |

| `api/migrations/00003_drop_pre_pickup_status.sql` | Drop `bookings.pre_pickup_status` column (pre-release cleanup) |
| `api/internal/db/queries/bookings.sql` | Remove `SetPrePickupStatus`, `GetPrePickupStatus`, `NoItemsPickedUp` queries; run `sqlc generate` |

No new API endpoints needed (uses existing `addBookingItems`, `removeBookingItem`, and `GET /api/v0/issues`).

---

## Out of scope / backlog

- Merge bookings
- Personal booking deprecation / creator name display
- Return flow UX (separate concern, not changed here)
- Reported unusable items during pickup for individually-tracked items (already works: Byt / Hämtad ändå buttons exist)


## Feedback:
- We currently both urge users to create a new booking and to add items. Adding items should be put up top in the ui. The picker is good but could open as a popup instead of now in lower edge of display. Maybe we could either search in free-text or choose first a category and then show everything in that category. We can remove the "Vill du lägga till mer utrustning? Skapa en ny bokning.". When searching, also non-approved items should show, but not possible to select (maybe when pressing them user should be urges to create a new booking and send to approval)
- All buttons should be up top. When in pickup we don't need a starta återlämning button but a simple Tillbaka button would be great, moving you to the booking overview instead. In the booking overview we can then see what items are picked up so far, and move into either pickup or return flow
- 