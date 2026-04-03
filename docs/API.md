# API Reference — v0 (pre-release)

Base URL: `/api/v0`

All endpoints under `/api/v0` require authentication. In dev mode, use the `X-Dev-Role-Override` header with a persona from `dev-personas.json`.

Endpoints marked 🔒 require the `equipment_manager` role.

---

## Health

### `GET /api/health`
No auth required.

**Response** `200`
```json
{"status": "ok"}
```

---

## Articles

### `GET /api/v0/articles`
List all articles for the group. Returns joined location and category names.

**Query parameters** (all optional):
- `search` — filter by commercial_name or common_name (case-insensitive substring match)
- `category_id` — filter by category UUID
- `location_id` — filter by location UUID
- `status` — filter by status (e.g. `ok`, `drying`, `under_repair`)

**Response** `200` — array of articles ordered by category, then commercial name, then common name.

### `GET /api/v0/articles/{id}`
Get a single article by ID.

**Response** `200` | `404`

### 🔒 `POST /api/v0/articles`
Create an article.

**Body**
```json
{
  "commercial_name": "Sibley",
  "common_name": "Sibley 1",
  "category_id": "uuid",
  "location_id": "uuid",
  "status": "ok",
  "individually_tracked": true,
  "requires_approval": false,
  "description": "",
  "instructions": "",
  "place": "Shelf 3"
}
```
Required: `common_name`, `category_id`, `location_id`. Status defaults to `ok`.

**Response** `201` | `400` | `403`

### 🔒 `PUT /api/v0/articles/{id}`
Update an article. Same body as create.

**Response** `200` | `400` | `403` | `404`

### 🔒 `DELETE /api/v0/articles/{id}`

**Response** `204` | `403` | `404`

### 🔒 `POST /api/v0/articles/import`
Import articles from CSV. Multipart form upload with field name `file`.

Auto-creates categories and locations that don't exist. See `docs/import-example.csv` for the expected format.

**Response** `200`
```json
{
  "imported": 1024,
  "skipped": 0,
  "errors": []
}
```

### `GET /api/v0/articles/availability`
Check available article counts grouped by commercial_name for a date range.

**Query parameters**:
- `start_date` (required) — ISO date (e.g. `2026-06-01`)
- `end_date` (required) — ISO date (e.g. `2026-06-05`)
- `category_id` — filter by category UUID
- `location_id` — filter by location UUID
- `bookable_only` — `true` (default) hides items requiring approval, `false` shows all

Results are grouped by commercial_name + location. Same product in different locations shows as separate groups.

**Response** `200`

### `GET /api/v0/articles/availability/articles`
List individual available articles for a date range. Used for swap selection during pickup.

**Query parameters**:
- `start_date` (required) — ISO date
- `end_date` (required) — ISO date
- `exclude_booking_id` — exclude items already in this booking from the unavailable set
- `commercial_name` — filter by commercial name

**Response** `200` — array of individual articles with `id`, `commercial_name`, `common_name`, `location_name`, `place`
```

---

## Bookings

### `GET /api/v0/bookings`
List bookings visible to the current user (own bookings + unit bookings).

**Response** `200`

### `POST /api/v0/bookings`
Create a draft booking.

**Body**
```json
{
  "start_date": "2026-06-01",
  "end_date": "2026-06-05",
  "used_by_unit_id": "uuid or null",
  "used_by_external": "string or null",
  "used_by_external_contact": "string or null",
  "notes": ""
}
```
Required: `start_date`, `end_date`.

**Response** `201` | `400`

### `GET /api/v0/bookings/{id}`
Get booking with its items (including article details).

**Response** `200` | `404`
```json
{
  "booking": { ... },
  "items": [
    {"id": "uuid", "commercial_name": "Sibley", "common_name": "Sibley 1", "location_name": "Hajkförrådet", ...}
  ]
}
```

### `PUT /api/v0/bookings/{id}`
Update a booking. Allowed on draft, submitted, approved, confirmed, and picked_up bookings. Access: creator, unit leaders, or equipment manager.

All fields are optional — only provided fields are updated. If dates change, all existing items are re-validated against availability.

**Body**
```json
{
  "start_date": "2026-06-02",
  "end_date": "2026-06-06",
  "used_by_unit_id": "uuid or null",
  "used_by_external": "string or null",
  "used_by_external_contact": "string or null",
  "notes": "Updated notes"
}
```

**Response** `200` | `400` | `403` | `404` | `409` (items not available for new dates)

### `POST /api/v0/bookings/{id}/items`
Add articles to a booking by commercial_name and quantity. Eagerly assigns specific available articles. Allowed on editable bookings (not returned/cancelled). Access: creator, unit leaders, or equipment manager.

**Body**
```json
{"commercial_name": "Sibley", "quantity": 2, "location_name": "Hajkförrådet"}
```
`location_name` is optional — if omitted, assigns from any location.

**Response** `201` | `400` | `404` | `409` (not enough available)

### `DELETE /api/v0/bookings/{id}/items/{itemId}`
Remove an item from an editable booking. Access: creator, unit leaders, or equipment manager.

**Response** `204` | `400` | `403` | `404`

### `POST /api/v0/bookings/{id}/submit`
Submit a draft booking. Auto-confirms if no articles require approval (or if user is project_leader). Otherwise transitions to `submitted` awaiting manager approval.

**Response** `200` | `400` | `404`

### `POST /api/v0/bookings/{id}/cancel`
Cancel a booking. Drafts are deleted entirely (returns 204). Other bookings transition to `cancelled` (returns 200). Cannot cancel returned or already cancelled bookings.

**Response** `200` | `204` | `400` | `403` | `404`

### `POST /api/v0/bookings/{id}/copy`
Create a new draft booking with the same unit, notes, and items as the source. Dates are set to today + 7 days as placeholders. Items that no longer exist are silently skipped.

**Response** `201`
```json
{
  "booking": { ... },
  "items_copied": 5,
  "items_total": 5
}
```

### `POST /api/v0/bookings/{id}/pickup`
Transition a confirmed or approved booking to `picked_up`. Access: creator, unit leaders, or equipment manager.

**Response** `200` | `400` | `403` | `404`

### `PUT /api/v0/bookings/{id}/items/{itemId}/pickup`
Set the pickup status for a single booking item. Booking must be in `picked_up` status. Access: creator, unit leaders, or equipment manager.

**Body**
```json
{"pickup_status": "picked_up"}
```
Valid values: `picked_up`, `not_available`.

**Response** `200` | `400` | `403` | `404`

### `POST /api/v0/bookings/{id}/items/{itemId}/swap`
Replace the article on a booking item during pickup. The new article must be available for the booking's date range. Sets pickup_status to `swapped`. Booking must be in `picked_up` status. Access: creator, unit leaders, or equipment manager.

**Body**
```json
{"new_article_id": "uuid"}
```

**Response** `200` | `400` | `403` | `404` | `409` (article not available)

### `POST /api/v0/bookings/{id}/return`
Transition a picked_up booking to `returned`. All items must have a final return status (not null, not pending, not drying). Access: creator, unit leaders, or equipment manager.

**Response** `200` | `400` | `403` | `404`

### `PUT /api/v0/bookings/{id}/items/{itemId}/return`
Set the return status for a single booking item. Booking must be in `picked_up` status. Access: creator, unit leaders, or equipment manager.

Side effects:
- `delayed` — no article status change, item stays on loan
- `broken` — sets article status to `reported_unusable`, creates issue report
- `lost` — sets article status to `archived`, creates issue report
- `returned_ok` — sets article status back to `ok`

When all items have a final return status (not pending/delayed), the booking auto-transitions to `returned`.

**Body**
```json
{
  "return_status": "returned_ok",
  "expected_return_date": "2026-06-10",
  "notes": "Optional, used as issue description for broken/lost"
}
```
Valid values: `returned_ok`, `delayed`, `broken`, `lost`, `""` (undo). `expected_return_date` required when status is `delayed`.

**Response** `200` | `400` | `403` | `404`

---

## Units

### `GET /api/v0/units`
List all units for the group.

**Response** `200`

### 🔒 `POST /api/v0/units`
```json
{"name": "Yggdrasil"}
```
**Response** `201` | `400` | `403`

---

## Locations

### `GET /api/v0/locations`
List all locations for the group, ordered by sort_order.

**Response** `200`

### 🔒 `POST /api/v0/locations`
```json
{"name": "Hajkförrådet", "sort_order": 1}
```
**Response** `201` | `400` | `403`

### 🔒 `PUT /api/v0/locations/{id}`
**Response** `200` | `400` | `403` | `404`

### 🔒 `DELETE /api/v0/locations/{id}`
**Response** `204` | `403` | `404`

---

## Categories

### `GET /api/v0/categories`
List all categories for the group, ordered by sort_order.

**Response** `200`

### 🔒 `POST /api/v0/categories`
```json
{"name": "Sova", "parent_id": null, "sort_order": 1}
```
**Response** `201` | `400` | `403`

### 🔒 `PUT /api/v0/categories/{id}`
**Response** `200` | `400` | `403` | `404`

### 🔒 `DELETE /api/v0/categories/{id}`
**Response** `204` | `403` | `404`

---

## Error format

All errors return:
```json
{"error": "error_key"}
```

Error keys are short English strings (e.g. `"invalid id"`, `"article not found"`, `"forbidden"`). The frontend maps these to translated user-facing messages.

---

## Authentication

In production: `Authorization: Bearer <jwt>` header with a valid Keycloak token.

In dev mode (`DEV_MODE=true`): `X-Dev-Role-Override: <persona>` header. Available personas:
- `leader-yggdrasil` — leader, unit Yggdrasil
- `leader-orneerna` — leader, unit Ornéerna
- `project-leader` — can book without approval
- `equipment-manager` — full admin
- `leader-and-manager` — combined roles
