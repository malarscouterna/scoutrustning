# UX Revamp - Dashboard + Cart

## Summary

Replace tab-based navigation with a dashboard landing page. Turn bookings into a floating cart. Simplify navigation to logo + back arrow.

## UI conventions

- **Navigation links**: `<scout-button>` with `type="link"` and chevron icon (`iconPosition="after"`). Variant varies by context (primary for CTAs, outlined for secondary actions, text for inline links).
- **Expandable sections**: plain text with a small ▲/▼ arrow. Consistent across dashboard ("Visa avslutade") and browse (group expand).
- **Icons**: Material Symbols Outlined from Google Fonts. Loaded in `app.html`:
  ```html
  <link rel="stylesheet" href="https://fonts.googleapis.com/css2?family=Material+Symbols+Outlined:opsz,wght,FILL,GRAD@24,400,0,0&icon_names=account_circle,search,arrow_back,shopping_bag" />
  ```
- **Scout components**: use `scout-button`, `scout-list-view`, `scout-list-view-item` (with `action="chevron"` for navigation items), `scout-card` where they fit. See `@scouterna/ui-webc`.

## Navigation

Minimal sticky top bar, all screen sizes:

```
[Logo → /]    ................    [← Tillbaka]
```

- Logo: group logo, links to `/` (dashboard).
- Back arrow (`arrow_back`): shown on all pages except `/`. Goes back (history or parent route).

No bottom bar. Browse, guide, profile, and settings are reached from the dashboard.

## Dashboard (`/`)

### Top section (full width)

Four `scout-button` CTAs, inline (wraps to 2×2 on narrow screens):
- **"Boka utrustning"** → `/book` (variant: primary)
- **"Bläddra utrustning"** → `/browse` (variant: outlined, `search` icon)
- **"Inställningar"** → `/profile` (variant: outlined, `account_circle` icon)
- **"Användarguide"** → `/guide` (variant: text)

All with chevron icon after text.

### Bokningar (left column on md+, first on mobile)

1. **Draft bookings** - cards with dates, team, item count. Tap → `/book?id={id}`, activates as cart.
2. **Pending approvals** (manager) - highlighted if count > 0. Each → `/bookings/[id]` with chevron. "Visa alla →" → `/bookings?filter=pending`.
3. **Active bookings** - status submitted/approved/confirmed/picked_up. Each → `/bookings/[id]` with chevron.
4. **"Visa avslutade" ▼** - expandable, lazy-loaded (returned/cancelled).
5. **"Visa alla bokningar"** → `/bookings` (scout-button, outlined, chevron).

### Ärenden (right column on md+, second on mobile)

1. **Issues I reported** - my articles with issue status. Name, status, latest comment. Each → `/articles/[id]` with chevron.
2. **Active issues** (manager) - the 5 most recent articles with reported/lost/under_repair status. Keeps the dashboard lightweight.
3. **"Visa avslutade" ▼** - expandable (resolved issues).
4. **"Visa alla ärenden"** → `/issues` (scout-button, outlined, chevron).

### Footer

Group name, user name. Logout link.

## Floating cart

FAB (floating action button) with expandable panel. Visible when a draft booking is active in localStorage.

### FAB (collapsed)

- Fixed bottom-right, 16px from edges
- ~56px circle, primary color, shadow
- `shopping_bag` icon + red badge with item count
- Tap to toggle panel

### Panel (expanded)

Slides up from FAB. Stays on current page.

```
┌─────────────────────────────────────┐
│ 12 jun - 15 jun · Yggdrasil         │
├─────────────────────────────────────┤
│ Sibley         Hajkf.  [−] 3 [+] × │  ← scrollable
│ Stormkök       Ladan   [−] 2 [+] × │     max-height ~50vh
│ Liggunderlag   Kallf.  [−] 8 [+] × │
│ Tältlampa LED  Hajkf.  [−] 8 [+] × │
├─────────────────────────────────────┤
│ [Visa bokning ›]        [Skicka ›]  │
└─────────────────────────────────────┘
```

- **Header**: dates + team (one line, read-only).
- **Item rows**: commercial name (tappable → `/articles/[id]`), location, [−]/[+] to adjust quantity, × to remove. Compact - no images, no descriptions.
- **[−]/[+]**: adjusts quantity for that commercial_name + location group. [−] at 1 removes the item. [+] adds one more (calls `addBookingItems` with quantity 1). Disabled when no more available.
- **"Visa bokning"** → `/book?id={activeId}` (full cart management). Scout-button with chevron.
- **"Skicka"** → submits directly, clears cart. Scout-button, primary.
- Tap FAB again or tap outside to close.
- Mobile: semi-transparent backdrop when open.

### Cart state

`$lib/stores/cart.ts` - localStorage key `active-booking-id`.

- Draft bookings are server-side. localStorage tracks which one is "active" in the UI.
- Multiple drafts can exist. Dashboard shows all; tapping one activates it.
- Submitting or cancelling clears the store.
- Switching devices: dashboard shows drafts, tap to activate.

## Cart management (`/book`)

### No active cart

Form: start date, end date, team, notes → "Skapa bokning" → creates draft, stores in cart, auto-redirects to `/book?id={newId}` (same page, now in cart management mode). User sees their empty cart and can tap "Lägg till utrustning ›" to go browse.

### Active cart (`?id={bookingId}`)

1. Editable: dates, team, notes.
2. Item list grouped by commercial_name + location. Each group shows name, quantity, location. Expandable (▼) to show full article details: images, description, instructions, approval level. Reuses the same detail rendering as browse page expanded groups.
3. Date change conflict handling: re-checks availability, flags conflicting items (warning indicator on affected groups). User removes them or reverts dates.
4. Actions: "Lägg till utrustning" (scout-button, chevron → `/browse`), "Skicka bokning" (scout-button, primary), "Avbryt bokning" (scout-button, danger).

This is the full review page - where you inspect what you're booking before submitting. The cart panel is the compact quick-access view; `/book` shows everything.

## Browse page (`/browse`) - cart integration

When a cart is active, the browse page switches data source and gains add-to-cart controls.

| | No cart | Cart active |
|---|---|---|
| Data | `listArticles` (current status) | `checkAvailability(startDate, endDate)` |
| Counts | "X/Y st" (available/total) | "X kvar" (for date range) |
| Expand | Articles with status, report, history | Availability + "Lägg till" + quantity |
| Edit mode | Available | Available (coexists) |
| Batch mode | Available | Available (coexists) |

All existing browse features preserved. AvailabilityPicker logic layered on when cart active.

### Cart mode indicator

Sticky banner below top bar when cart is active:

```
┌─────────────────────────────────────────────────────┐
│ 🛒 Bokar för Yggdrasil, 12-15 jun · 3 artiklar  [×]│
└─────────────────────────────────────────────────────┘
```

- Colored background (blue-50, blue border).
- **[×]**: deactivates cart (clears localStorage, reverts to normal browse). Draft stays in DB.
- Tapping the text → `/book?id={activeId}`.
- Critical for making the mode switch visible and reversible.

## Booking interactions from dashboard

| Booking status | Tap on dashboard | Why |
|---|---|---|
| draft | → `/book?id={id}`, activates as cart | Continue building the booking |
| submitted | → `/bookings/[id]` | View detail, don't accidentally edit. "Redigera" button on detail page if needed. |
| approved/confirmed | → `/bookings/[id]` | View detail, start pickup |
| picked_up | → `/bookings/[id]` | View detail, do return |
| returned/cancelled | → `/bookings/[id]` | View detail (in "Visa avslutade" section) |

Only drafts activate the cart. Everything else goes to the read-only detail page, which already has "Redigera" for when you need to change things.

## Article detail (`/articles/[id]`) - cart integration

When a cart is active: "Lägg till i bokning" scout-button with chevron. Uses existing `addBookingItems` API. ~15 lines added.

## Route changes

| Route | Change |
|---|---|
| `/` | Rewrite as dashboard |
| `/browse` | Add cart-aware mode |
| `/book` | Remove inline AvailabilityPicker, add cart display with conflict indicators |
| `/bookings` | Minor: back-link to `/` |
| `/issues` | Minor: back-link to `/` |
| `/articles/[id]` | Minor: add-to-cart button when cart active |
| All others | Unchanged |

## File changes

### New
| File | ~Lines |
|---|---|
| `$lib/components/BookingCard.svelte` | 30 |
| `$lib/components/IssueCard.svelte` | 30 |
| `$lib/components/FloatingCart.svelte` | 150 |
| `$lib/stores/cart.ts` | 30 |
| `routes/+page.server.ts` | 30 |

### Modified
| File | Change |
|---|---|
| `routes/+page.svelte` | Rewrite as dashboard (~150 lines) |
| `routes/+layout.svelte` | Replace nav with top bar + FloatingCart |
| `routes/browse/+page.svelte` | Cart detection + availability mode (~60-80 lines) |
| `routes/book/+page.svelte` | Remove AvailabilityPicker, add conflict indicators |
| `routes/articles/[id]/+page.svelte` | Add-to-cart button (~15 lines) |
| `routes/bookings/+page.svelte` | Back-link |
| `routes/issues/+page.svelte` | Back-link |
| `web/src/app.html` | Material Symbols font link |
| `smoke-test.sh` | Update route checks |

### Unchanged
All `$lib/components/`, `$lib/api/client.ts`, all Go API code, all database schema.

## Size estimate

~450-550 lines new code (FloatingCart is larger with +/- controls). ~80 lines removed. No API changes, no DB changes. Frontend-only.

## Implementation order

1. Extract BookingCard + IssueCard from existing pages
2. Cart store (`$lib/stores/cart.ts`)
3. FloatingCart component + add to layout
4. Layout: replace nav with minimal top bar
5. Dashboard: `+page.svelte` + `+page.server.ts`
6. Book page: remove AvailabilityPicker, add cart display + conflict handling
7. Browse page: cart-aware mode + banner
8. Article detail: add-to-cart button
9. Polish: back-links, smoke test, scout-button migration on CTAs

## Documentation updates

After implementation, update these docs:

| Document | What to update |
|---|---|
| `docs/SPEC.md` | User flows section (booking flow now uses cart + browse instead of dedicated /book page). Navigation description. Add UPDATE notes on changed steps. |
| `docs/API.md` | No changes (no new endpoints). |
| `docs/guide.md` | Rewrite user-facing instructions: new dashboard, how to book (cart flow), how to browse, where to find settings. Screenshots if any. |
| `docs/BACKLOG.md` | Remove this item if listed. Add any deferred items (issue assignment, draft cleanup). |
| `docs/accomplished.md` | Log the revamp as completed work. |
| `.amazonq/rules/project-context.md` | Update project structure (new files), navigation description, user flow description. |
| `README.md` | Update "Currently implements" list if navigation/UX is mentioned. |

## Open questions

1. **Date change conflicts**: flag unavailable items and let user decide, or auto-remove with warning?
2. **DevPersonaSwitcher position**: currently bottom-right, overlaps FAB. Move to top-right or bottom-left.


## Feedback

All items below resolved unless marked **OPEN**.

| # | Description | Status |
|---|---|---|
| 1 | Submitted bookings duplicated in both "Väntar" and "Aktiva" sections | Done - frontend fix: exclude pendingApprovals IDs from active list |
| 2 | +/- buttons in booking view (was only +) | Done - BookingItemsList now has compact -/count/+ controls at group level |
| 3 | Badge counter not animating on updates; make it red and larger | Done - red badge with pop/bump CSS keyframe animations |
| 4 | Remove "Ej tillgänglig / Lägg till i bokning" from article expansion (inline button covers it) | Done - removed from expand panel in browse |
| 5 | Move status counters below image; Visa artikelsida/Redigera last | Done - reordered non-individually-tracked expansion |
| 6 | Rename "Bläddra utrustning" to "Visa utrustning" | Done |
| 7 | Back navigation goes to list instead of home; label should say destination | Done - replaced history.back() with named destination links ("Hem" / "Utrustning") |
| 8 | Issues page: replace mine/all checkbox with two sections; add "Visa avslutade" | Done - Mina ärenden / Övriga ärenden sections; "Visa avslutade" checkbox adds ok status |
| 9 | Cart persists after persona switch | Done - cart.clear() called before persona reload |
| 10 | Item names in floating cart should be links to article | Done - linked via article_id |
| 11 | Sort by location in floating cart and booking view; location as section header; show category in cart | Done - both FloatingCart and BookingItemsList grouped by location, sorted by category then name |
| 12 | No "Lägg till en till" text, just +/-; quantity-tracked items grouped by status | Done - compact controls + status grouping for non-individually-tracked items |
| 13 | Persona switcher collides with back button | Done - moved to top-14 (below nav bar) |
| 14 | Issues overview: show date issue was reported and date of last update | **OPEN** - needs backend: Article response lacks issue lifecycle timestamps. Requires adding issue_reported_at and last_event_at to the API. |
| 15 | Cart badge doesn't update until cart is opened | Done - added refresh signal to cart store, FloatingCart listens and reloads. All add/remove operations trigger refresh. |
| 16 | Missing minus button in /browse when item is in cart | Done - inline [−] count [+] controls shown when item in cart, single [+] when not |
| 17 | Item removal priority: keep ok items as long as possible | Done - removal prioritizes: reported_usable > under_repair > incoming > ok |
| 18 | Articles with approval_level low/high showing as unavailable (count 0) in browse cart mode | Done - availability endpoint was defaulting bookable_only=true, filtering out non-none approval articles; flipped default to false |

## Feedback final touches

| # | Description | Status |
|---|---|---|
| 1 | Button to create a booking from the Bookings view | Done - "Ny bokning" button in bookings page header |
| 2 | Standardize nav: Hem/Bokningar always top-right; removed per-page back arrow | Done - layout always shows Hem and Bokningar links; removed redundant "Tillbaka" from booking detail |
| 3 | Approval level badges in browse (none=none, low=amber, high=red); trusted+ sees "Förgodkänd" on low | Done - badge on group row; label is context-aware: uses cart team's access level when cart is active, falls back to user max_access; colors hardcoded as inline styles (Tailwind purge workaround, see backlog); sorting by approval level is backlog |
| 4 | Bokningar page: show all bookings sorted by status then start date | **OPEN** - not yet implemented |
| 5 | Browse expand reorder: image, then counts, then text descriptions | Done - text descriptions now rendered after article list/state rows |
| 6 | Issue links for non-ok items in browse; article page shows sibling items | **OPEN** - rethinking issue reporting flow together with user |

## Known issues

| # | Description |
|---|---|
| 1 | Smoke test: `Profile (view-only)` returns 500. Not yet root-caused - view-only persona has no teams and max_access=view. May be pre-existing. |