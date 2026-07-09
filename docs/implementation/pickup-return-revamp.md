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

1. ✅ **Backend** - Remove auto-revert in `UpdateItemPickup` (`bookings.go`). (Issue 3)
2. ✅ **Backend** - Block PUT booking edit if status is `picked_up`. (Issue 4)
3. ✅ **Frontend `+page.svelte`** - Lift `reload` into the parent, add poll/reload race guard (Issue 13). Hide Redigera for `picked_up` (Issue 4). `pickupMode`/`returnMode` state-based navigation with overview hub (Issues 5, F2). Remove personal booking label (Issue 6). Fix `reopenBooking` (Issue 12). Floating bottom bar with "Tillbaka" + "Lägg till utrustning" (F4). Cart warning on pickup start (F5).
4. ✅ **Frontend `PickupChecklist.svelte`** - Split quantity groups by status category, partial pickup visibility, "Ta bort från bokning", issue-detail expand for reported_usable (Issues 1, 2). Simplified `markQuantityGroup`: removed `loadExtraAvailability`, uses reload return value (Issues 7, 8, Proposal A). "Byt" crash fix (F3).
5. ✅ **Frontend `ReturnChecklist.svelte`** - Clear active form on external update (Issue 9). Guard Slutför button (Issue 10). Reset count input on form open (Issue 11).
6. ✅ **Frontend `AddItemSheet.svelte`** - Modal overlay, free-text search + category browse, non-approved items shown disabled, calls `addBookingItems` on confirm (Proposal A, F1).

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
- The byt button should still show something even when nothing can be changed into, now it shows "available is not iterable" - but it should be able to handle that case gracfully. it should not auto-assume that we picked something up nonetheless if error reported.
- Actually, both the tillbaka button and the lägg till utrustning button should probably be floating at bottom of our page.
- If we start a pickup flow, we should not at same time be able to have a cart up - it should be set into draft, but maybe show some warning when going into booking
- partial return and partial pickup should be just as natural to do. Complete with tillbaka button to show overview of booking but without action alternatives.

---

## Feedback implementation plan

### F1. AddItemSheet as floating button + modal popup

**Files:** `PickupChecklist.svelte`, `ReturnChecklist.svelte`, `AddItemSheet.svelte`, `+page.svelte`

- Remove the inline `+ Lägg till utrustning` button from the bottom of `PickupChecklist` and `ReturnChecklist`. The button moves to a shared floating bottom bar in `+page.svelte` (see F4).
- Restyle `AddItemSheet` as a centered modal (fixed overlay) rather than a bottom-of-list insertion.
- Add category browse alongside free-text search: first show a category list (derived from `commercial_name` groupings or a dedicated endpoint); selecting a category filters results. Free-text search also works directly.
- Non-approved items appear in search/category results but are visually disabled (opacity, no selection). Tapping them shows a small inline hint: "Kräver godkännande - skapa en ny bokning" with a link to `/browse`.
- Articles with zero available quantity (fully booked for the date range, or all units unavailable) also appear in results but are disabled and labelled "Ej tillgänglig". Tapping them shows "Skapa en ny bokning för ett annat datum" with a link to `/browse`.
- Remove the `"Vill du lägga till mer utrustning? Skapa en ny bokning."` hint from `+page.svelte` (the `picked_up` else-branch under the Redigera button).

### F2. Navigation redesign: Tillbaka + overview as hub

**File:** `+page.svelte`

**New flow:**
- `+page.svelte` is the overview hub. When `booking.status === 'picked_up'` and no pickup/return mode is active, show the booking overview with item pickup statuses and two action buttons: "Fortsätt utlämning" (if any items lack `pickup_status`) and "Påbörja återlämning".
- `pickupMode` and `returnMode` are local boolean flags (replacing the old `pickupStarted` + `showReturn`). Only one can be active at a time.
- In pickup mode: render `PickupChecklist`. Floating bottom bar (F4) shows "Tillbaka" and "Lägg till utrustning".
- In return mode: render `ReturnChecklist`. Floating bottom bar shows "Tillbaka" and "Lägg till utrustning".
- "Tillbaka" exits the current mode and returns to overview without any API call. Partial progress is preserved.
- Remove the `Starta återlämning` button that was rendered below `PickupChecklist`.
- Overview item list: show each item with its `pickup_status` badge (Hämtad / Ej hämtad) so the user can see progress at a glance. Re-use `BookingItemsList` or inline the status display.

### F3. Fix "Byt" crash when no swap candidates

**File:** `PickupChecklist.svelte` - `startSwap()`

- The `listAvailableArticles` call returns an array; if the API returns an error or non-iterable, the spread `[current, ...available]` crashes. Wrap in a try/catch that sets `swapCandidates = [current]` on failure so the UI shows "Inga tillgängliga ersättare" gracefully.
- If `available` is empty (not an error, just no candidates), the UI already handles `swapCandidates.length <= 1 && swapCandidates[0]?.is_current` - verify this path renders correctly and does not auto-confirm pickup.
- Do not call `markPickup` or `confirmSwap` automatically when the swap list is empty or errors.

### F4. Floating bottom bar in pickup/return modes

**File:** `+page.svelte`

- When `pickupMode` or `returnMode` is active, render a sticky bottom bar fixed to the bottom of the viewport (e.g. `fixed bottom-0 left-0 right-0 bg-white border-t px-4 py-3 flex gap-3`).
- In pickup mode bar contains: "Tillbaka" button (left, secondary style) and "Lägg till utrustning" button (right, primary style).
- In return mode bar contains only: "Tillbaka" button. Adding equipment is not allowed during return.
- "Lägg till utrustning" opens `AddItemSheet` (state lives in `+page.svelte`, passed as prop or via context).
- Add `pb-20` (or equivalent) to the checklist wrapper so content is not obscured by the bar.

### F5. Cart - draft warning when starting pickup

**File:** `+page.svelte`, `$lib/stores/cart.svelte`

- On `startPickup()`, before calling `api.pickupBooking`, check if the cart store has an active booking ID that differs from `booking.id`.
- If yes: show a confirmation dialog (native `confirm()` or inline warning): "Du har en aktiv varukorg. Den kommer att sättas till utkast för att du ska kunna påbörja utlämning. Vill du fortsätta?"
- On confirm: call `api.updateBooking(cartBookingId, { status: 'draft' })` (or whichever endpoint sets a booking back to draft - check if this exists; if not, use `cancelBooking` is too destructive, so may need a dedicated revert-to-draft endpoint or just clear the cart without API call). Clear the cart store (`activeBookingId = null`). Then proceed with `startPickup`.
- On cancel: abort, do not start pickup.
- **Note:** Check whether the API supports reverting a submitted/confirmed booking to draft. If not, just clear the cart store locally and note the booking remains in its current status (the cart UI will no longer show it as active).

### F6. Partial pickup and partial return - overview without action pressure

This is largely covered by F2 (Tillbaka returns to overview). Additional notes:

- The overview does not show a "you must complete" prompt. It shows current state neutrally.
- "Fortsätt utlämning" only appears if there are items without `pickup_status` (partial pickup possible).
- "Påbörja återlämning" always appears when `booking.status === 'picked_up'` (per Proposal E - already accepted).
- If all items are picked up, "Fortsätt utlämning" is hidden (nothing left to pick up).
- If the user has already started a return (some items have `return_status`), change "Påbörja återlämning" to "Fortsätt återlämning".

---

### Feedback implementation order

1. **F3** - Fix "Byt" crash (isolated, low risk)
2. **F2 + F4** - Navigation redesign + floating bar (core UX change)
3. **F1** - AddItemSheet modal + category browse + non-approved items
4. **F5** - Cart warning on pickup start
5. **F6** - Verify overview neutrality (mostly falls out of F2)