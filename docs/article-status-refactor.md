# Article status & availability refactor

## Problem

Article status currently mixes two orthogonal concerns:
1. **Condition** — is the item OK, broken, under repair?
2. **Booking state** — is it reserved, out on loan?

Statuses like `booked`, `loaned`, `drying` conflate these. The availability calculation already computes booking state from `booking_items` + date ranges, making stored booking state redundant and error-prone.

Users need to understand *why* something is unavailable: is it booked by someone else? Under repair? Not arrived yet? And *when* it will be back.

## Design

### `articles.status` — condition (stored, manually managed)

| Value | Meaning | Bookable? |
|---|---|---|
| `ok` | Working, no issues | Yes |
| `reported_usable` | Flagged issue, still usable | Yes |
| `incoming` | Ordered/planned, not yet in storage | Yes (for future dates past `expected_available_date`) |
| `reported_unusable` | Flagged issue, not usable | No |
| `under_repair` | Out for repair | No (until `expected_available_date`) |
| `lost` | Gone | No |
| `archived` | Retired from inventory | No |

Order in UI: `ok, reported_usable, incoming, reported_unusable, under_repair, lost, archived`

Set by: users (reported_*), equipment managers (all), CSV import (ok/incoming).

### `articles.expected_available_date` — when will it be usable again? (nullable)

Replaces `drying_until`. Used for:
- `incoming` — expected delivery date
- `under_repair` — expected repair completion date

When set, the article becomes bookable for date ranges starting after this date. The browse/inventory view shows "beräknas tillgänglig [date]".

### Availability — computed, never stored

Availability is always computed for a specific point in time (or date range) by combining:

1. **Article condition** (`status`) — is it in a bookable condition?
2. **Booking overlaps** — is it assigned to an active booking for the requested dates?
3. **Expected dates** — for `incoming`/`under_repair`, is the requested date range after `expected_available_date`?

This is what the `AvailableArticles` query already does. The inventory/browse page uses the same logic with date = today.

### Computed availability states (for display, not stored)

| State | How determined | Display |
|---|---|---|
| Tillgänglig | Bookable condition + no overlapping booking | Green |
| Reserverad | Bookable condition + assigned to confirmed/approved booking overlapping the date | Blue |
| Utlånad | Assigned to picked_up booking + not yet returned | Purple |
| Ej tillgänglig | Non-bookable condition (`reported_unusable`, `under_repair`, `lost`, `archived`) | Grey/Red |
| Inkommande | `status = incoming`, before `expected_available_date` | Blue outline |

### Why this is better than a stored column

- **No sync issues**: availability is always correct because it's computed from source-of-truth data
- **Time-aware**: same article can be "available now, reserved next week, available again in August" — a column can only represent one point in time
- **Explains why**: the query can return *why* something is unavailable (which booking, what condition, when it's expected back)
- **Single code path**: browse page and booking page use the same availability logic

### What the user sees

**Browse page (inventory, date = today):**
- Stormkök: "4/6 tillgängliga" → expand → "Stormkök 1: OK, Stormkök 2: OK, Stormkök 3: OK — Utlånad (Yggdrasil, tillbaka 5 jun), Stormkök 4: Rapporterad användbar, Stormkök 5: Rapporterad ej användbar"
- Tältlampa LED: "×5/8 tillgängliga" → expand → "5 OK, 1 Rapporterad användbar, 1 Rapporterad ej användbar, 1 Under reparation (beräknas klar 10 jun)"

**Booking page (date = selected range):**
- Same availability data but for the requested date range
- "3 Sibley tillgängliga 15-20 jun" (the other 2 are reserved by another booking that overlaps)

### Availability query changes

The existing `AvailableArticles` query filters `a.status IN ('ok', 'reported_usable')`. This needs to expand:

For **booking availability** (can I book this for date range X-Y?):
- `status IN ('ok', 'reported_usable')` — currently bookable
- OR `status = 'incoming' AND expected_available_date <= start_date` — will be available by then
- OR `status = 'under_repair' AND expected_available_date IS NOT NULL AND expected_available_date <= start_date` — will be fixed by then
- AND not assigned to an overlapping active booking

For **inventory view** (what's the state right now?):
- Show ALL articles (all statuses)
- Compute availability per article: check if assigned to an active booking where `start_date <= today AND end_date >= today`
- Show condition (status) + availability (computed) + expected dates

### New inventory query

A new query (or enriched `ListArticles`) that returns articles with their current booking context:

```sql
SELECT a.*,
    l.name AS location_name,
    c.name AS category_name,
    -- Current booking info (if any)
    cur_booking.id AS current_booking_id,
    cur_booking.status AS current_booking_status,
    cur_booking.end_date AS current_booking_end_date,
    cur_unit.name AS current_booking_unit_name
FROM articles a
JOIN locations l ON a.location_id = l.id
JOIN categories c ON a.category_id = c.id
LEFT JOIN LATERAL (
    SELECT b.id, b.status, b.end_date, b.used_by_unit_id
    FROM booking_items bi
    JOIN bookings b ON bi.booking_id = b.id
    WHERE bi.article_id = a.id
        AND b.group_id = a.group_id
        AND b.status IN ('confirmed', 'approved', 'picked_up')
        AND b.start_date <= CURRENT_DATE
        AND b.end_date >= CURRENT_DATE
        AND (bi.return_status IS NULL OR bi.return_status IN ('pending', 'delayed'))
    ORDER BY b.start_date
    LIMIT 1
) cur_booking ON true
LEFT JOIN units cur_unit ON cur_booking.used_by_unit_id = cur_unit.id
WHERE a.group_id = @group_id
    ...filters...
```

This gives us everything in one query: article condition + who has it + when it's coming back.

## Statuses removed from articles

- `booked` — computed from booking data (reserved)
- `loaned` — computed from booking data (picked_up booking)
- `drying` — replaced by `under_repair` + `expected_available_date`
- `new` — replaced by `incoming` (clearer: ordered but not yet in storage)

## Column changes

- **Remove**: `drying_until`
- **Add**: `expected_available_date date` (nullable) — used with `incoming` and `under_repair`
- **No `availability` column** — computed at query time

## Implementation steps

### 1. Migration
- Add `expected_available_date date` column (nullable)
- Migrate `drying_until` values to `expected_available_date`
- Drop `drying_until`
- Update status CHECK: remove `booked`, `loaned`, `drying`, `new`; add `incoming`
- Migrate existing data: `drying` → `ok`, `new` → `incoming`, `booked` → `ok`, `loaned` → `ok`

### 2. sqlc queries
- Update `UpdateArticleStatus` — replace `drying_until` with `expected_available_date`
- Add/update inventory query that joins current booking context (see SQL above)
- Update `AvailableArticles` to include `incoming`/`under_repair` with `expected_available_date` logic
- Remove status filter for `loaned`/`booked`/`drying` everywhere

### 3. Go handlers
- `UpdateStatus`: validate new status enum, handle `expected_available_date` for `incoming`/`under_repair`
- `UpdateItemPickup`: no article status change needed (booking data is the truth)
- `UpdateItemReturn`: set article status based on return condition (`ok`, `reported_*`, `lost`) — no availability column to update
- Remove any code that sets article status to `booked`/`loaned`/`drying`

### 4. API types
- Add `expected_available_date` to Article
- Add computed availability fields to article list response (current_booking_id, current_booking_status, current_booking_end_date, current_booking_unit_name)

### 5. Browse page
- Show condition (status badge) + computed availability (reserved/loaned indicator with booking info)
- Header count: articles with bookable condition (`ok`/`reported_usable`) AND not currently reserved/loaned
- Individually tracked expanded view: each item shows condition + "Utlånad till Yggdrasil, tillbaka 5 jun" or "Reserverad för Sommarläger 1-5 jul"
- Quantity tracked expanded view: status summary + "3 utlånade, 1 under reparation (beräknas klar 10 jun)"
- Expected dates shown for `incoming` and `under_repair`

### 6. Booking availability page
- Update to include `incoming`/`under_repair` articles that will be available by the booking start date
- Show why items are unavailable (booked by X, under repair until Y)

### 7. Issues page
- No changes needed (filters on condition statuses only)

### 8. Seed script
- Update to use new statuses
- Add `expected_available_date` examples for `under_repair` items

### 9. Tests
- Update status enum references
- Test that `incoming` articles with `expected_available_date` show as available for future bookings
- Test inventory query returns booking context

### 10. Docs
- Update SPEC.md data model
- Update API.md
- Update BACKLOG.md
- Update project-context.md

## Not in scope (deferred)

- Admin UI for managing `expected_available_date`
- Notifications when `expected_available_date` passes and item isn't back
- Automatic status transitions (e.g. `incoming` → `ok` when `expected_available_date` passes)
- Historical availability view (what was the state on date X?)
