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

### `PUT /api/v0/articles/{id}/status`
Update article status with an optional comment. Any user can set issue statuses (`reported_usable`, `reported_unusable`, `lost`) — comment is required for these. Manager-only statuses (`ok`, `under_repair`, `archived`, etc.) require the `equipment_manager` role. Logs an article event (`issue_reported` for issue statuses, `issue_resolved` when setting back to `ok`, `status_change` otherwise).

**Body**
```json
{
  "status": "reported_usable",
  "comment": "Tent has a tear in the fabric"
}
```
Required: `status`. `comment` required when reporting (reported_usable, reported_unusable, lost), optional for manager statuses.

**Response** `200` (updated article) | `400` | `403` | `404`

### `GET /api/v0/articles/{id}/events`
Get the event history for an article. Returns logged events (status changes, issue reports, resolutions, returns) ordered by most recent first.

**Query parameters** (all optional):
- `limit` — maximum number of events to return. When set, response includes `has_more` to indicate if more events exist beyond the limit.

**Response** `200`
```json
{
  "events": [{"id": "uuid", "event_type": "...", "description": "...", "metadata": {}, "actor_name": "...", "created_at": "..."}],
  "has_more": false
}
```

Event types: `status_change`, `issue_reported`, `issue_resolved`, `booked`, `picked_up`, `returned`, `note`.

---

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
List bookings visible to the current user. Leaders see their own bookings + bookings for their units/projects. Equipment managers see all bookings in the group.

**Response** `200`

### `POST /api/v0/bookings`
Create a draft booking. When `used_by_unit_id` is set, the user must be a member of that unit or project (name must appear in their token claims). Equipment managers are exempt from this check.

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

**Response** `201` | `400` | `403` (not a member of the unit/project)

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
Transition a confirmed or approved booking to `picked_up`. Saves the current status (`confirmed` or `approved`) as `pre_pickup_status` so it can be restored if all pickups are undone. Access: creator, unit leaders, or equipment manager.

**Response** `200` | `400` | `403` | `404`

### `PUT /api/v0/bookings/{id}/items/{itemId}/pickup`
Set the pickup status for a single booking item. Booking must be in `picked_up` status. Access: creator, unit/project members, or equipment manager. Logs a `picked_up` article event with the acting user.

Sending an empty string clears the pickup status (undo). If all items in the booking have their pickup status cleared, the booking automatically reverts to its pre-pickup status (`confirmed` or `approved`).

**Body**
```json
{"pickup_status": "picked_up"}
```
Valid values: `picked_up`, `lost`, `""` (undo).

**Response** `200` | `400` | `403` | `404`

### `POST /api/v0/bookings/{id}/items/{itemId}/swap`
Replace the article on a booking item during pickup. The new article must be available for the booking's date range. Sets pickup_status to `swapped`. Booking must be in `picked_up` status. Access: creator, unit/project members, or equipment manager. Logs a `picked_up` article event for the new article.

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

Side effects (all also log an article event with the acting user):
- `delayed` — no article status change, item stays on loan, logs `returned` event with `delayed` status
- `broken` — sets article status to `reported_unusable`, logs `issue_reported` event
- `lost` — sets article status to `archived`, logs `issue_reported` event
- `returned_ok` — sets article status back to `ok`, logs `returned` event

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

## Units & Projects

Units (e.g. "Yggdrasil") and projects (e.g. "Valborg 2026") are both stored in the `units` table, distinguished by a `type` field. Both can be assigned to bookings via `used_by_unit_id`. Membership comes from OIDC token claims.

### `GET /api/v0/units`
List all units and projects for the group, ordered by type then name.

**Response** `200`
```json
[
  {"id": "uuid", "name": "Yggdrasil", "type": "unit", ...},
  {"id": "uuid", "name": "Valborg 2026", "type": "project", ...}
]
```

### 🔒 `POST /api/v0/units`
```json
{"name": "Yggdrasil", "type": "unit"}
```
`type` defaults to `unit`. Valid values: `unit`, `project`.

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
- `project-leader` — project leader, project Valborg 2026 (books without approval)
- `equipment-manager` — full admin
- `leader-and-manager` — combined roles, unit Yggdrasil
- `other-group-leader` — leader in group 999 (Testkåren), for multi-tenancy testing
