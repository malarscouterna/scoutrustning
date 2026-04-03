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

**Query parameters** (required):
- `start_date` — ISO date (e.g. `2026-06-01`)
- `end_date` — ISO date (e.g. `2026-06-05`)

**Response** `200`
```json
[
  {"commercial_name": "Sibley", "available_count": 3, "requires_approval": false},
  {"commercial_name": "Stormkök", "available_count": 12, "requires_approval": false}
]
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
{"commercial_name": "Sibley", "quantity": 2}
```

**Response** `201` | `400` | `404` | `409` (not enough available)

### `DELETE /api/v0/bookings/{id}/items/{itemId}`
Remove an item from an editable booking. Access: creator, unit leaders, or equipment manager.

**Response** `204` | `400` | `403` | `404`

### `POST /api/v0/bookings/{id}/submit`
Submit a draft booking. Auto-confirms if no articles require approval (or if user is project_leader). Otherwise transitions to `submitted` awaiting manager approval.

**Response** `200` | `400` | `404`

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
