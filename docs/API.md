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
- `status` — filter by status (e.g. `ok`, `incoming`, `under_repair`)
- `mine` — `true` to only show articles linked to the user's bookings or issue reports
- `with_availability` — `true` to enrich each article with current booking context (who has it, when it's coming back)
- `date` — ISO date (e.g. `2026-06-15`), used with `with_availability=true`. Defaults to today. Shows booking state as of this date.

When `with_availability=true`, each article includes:
- `current_booking_id` — UUID of the active booking (confirmed/approved/picked_up) overlapping the date, or null
- `current_booking_status` — booking status (`confirmed`, `approved`, `picked_up`), or empty
- `current_booking_end_date` — when the booking ends, or null
- `current_booking_team_name` — name of the team using it, or null

Article `approval_level` values:
- `none` — freely bookable
- `low` — project leaders auto-approve, regular leaders need manager approval
- `high` — always needs manager approval (except for managers themselves)

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
  "approval_level": "none",
  "description": "",
  "instructions": "",
  "place": "Shelf 3",
  "purchase_date": "2024-03-15",
  "purchase_price": "1299.50",
  "manager_notes": "Internal note"
}
```
Required: `common_name`, `category_id`, `location_id`. Status defaults to `ok`. `approval_level` defaults to `none`. `purchase_date`, `purchase_price`, `manager_notes` are optional.

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
- `bookable_only` — `true` (default) hides items with `approval_level` != `none`, `false` shows all

Results are grouped by commercial_name + location. Same product in different locations shows as separate groups.

**Response** `200`

### `PUT /api/v0/articles/{id}/status`
Update article status with an optional comment. Any user can set issue statuses (`reported_usable`, `reported_unusable`, `lost`) — comment is required for these. Manager-only statuses (`ok`, `incoming`, `under_repair`, `archived`, etc.) require the `equipment_manager` role. Logs an article event (`issue_reported` for issue statuses, `issue_resolved` when setting back to `ok`, `status_change` otherwise).

Valid statuses: `ok`, `reported_usable`, `incoming`, `reported_unusable`, `under_repair`, `lost`, `archived`.

`expected_available_date` is only valid for `incoming` and `under_repair` statuses. When set, the article becomes bookable for date ranges starting on or after this date.

**Body**
```json
{
  "status": "under_repair",
  "comment": "Sent for repair",
  "expected_available_date": "2026-07-01"
}
```
Required: `status`. `comment` required when reporting (reported_usable, reported_unusable, lost), optional for manager statuses. `expected_available_date` optional, only for incoming/under_repair.

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

Event types: `status_change`, `issue_reported`, `issue_resolved`, `count_changed`, `booked`, `picked_up`, `returned`, `note`.

### `GET /api/v0/articles/{id}/group-events`
Get the aggregated event history for all articles in a quantity tracked group (matched by commercial_name + location). Returns events from all articles in the group, ordered by most recent first.

**Query parameters** (all optional):
- `limit` — maximum number of events to return. When set, response includes `has_more`.

**Response** `200` — same format as `/articles/{id}/events`, with additional `article_name` field per event.

---

### 🔒 `PUT /api/v0/articles/bulk`
Bulk update articles. Supports status change, location move, and archive with conflict detection.

For archive: checks active booking conflicts, auto-swaps where a replacement is available, returns unresolved conflicts.

**Body**
```json
{
  "article_ids": ["uuid1", "uuid2"],
  "status": "archived",
  "location_id": "uuid"
}
```
At least one of `status` or `location_id` required.

**Response** `200`
```json
{
  "updated": 5,
  "conflicts": [
    {
      "article_id": "uuid",
      "article_name": "Sibley 3",
      "booking_id": "uuid",
      "booking_dates": "2026-06-05 — 2026-06-08",
      "booking_team": "Yggdrasil"
    }
  ]
}
```

### 🔒 `POST /api/v0/articles/group-count`
Adjust the count of a quantity tracked article group. Creates or archives article records to match the new count. Logs a single `count_changed` event on the representative (oldest) article. Never archives the representative.

**Body**
```json
{
  "commercial_name": "Liggunderlag",
  "location_id": "uuid",
  "new_count": 12
}
```

**Response** `200`
```json
{"count": 12}
```
`409` with `cannot_reduce_count` if too many articles are in active bookings to reduce to the requested count.

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
List bookings visible to the current user. Leaders see their own bookings + bookings for their teams. Equipment managers see all bookings in the group.

**Response** `200`

### `POST /api/v0/bookings`
Create a draft booking. When `used_by_team_id` is set, the user must be a member of that team (name must appear in their token claims). Equipment managers are exempt from this check.

**Body**
```json
{
  "start_date": "2026-06-01",
  "end_date": "2026-06-05",
  "used_by_team_id": "uuid or null",
  "used_by_external": "string or null",
  "used_by_external_contact": "string or null",
  "notes": ""
}
```
Required: `start_date`, `end_date`.

**Response** `201` | `400` | `403` (not a member of the team)

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
Update a booking. Allowed on draft, submitted, approved, confirmed, and picked_up bookings. Access: creator, team members, or equipment manager.

All fields are optional — only provided fields are updated. If dates change, all existing items are re-validated against availability.

**Body**
```json
{
  "start_date": "2026-06-02",
  "end_date": "2026-06-06",
  "used_by_team_id": "uuid or null",
  "used_by_external": "string or null",
  "used_by_external_contact": "string or null",
  "notes": "Updated notes"
}
```

**Response** `200` | `400` | `403` | `404` | `409` (items not available for new dates)

### `POST /api/v0/bookings/{id}/items`
Add articles to a booking by commercial_name and quantity. Eagerly assigns specific available articles. Allowed on editable bookings (not returned/cancelled). Access: creator, team members, or equipment manager.

**Body**
```json
{"commercial_name": "Sibley", "quantity": 2, "location_name": "Hajkförrådet"}
```
`location_name` is optional — if omitted, assigns from any location.

**Response** `201` | `400` | `404` | `409` (not enough available)

### `DELETE /api/v0/bookings/{id}/items/{itemId}`
Remove an item from an editable booking. Access: creator, team members, or equipment manager.

**Response** `204` | `400` | `403` | `404`

### `POST /api/v0/bookings/{id}/submit`
Submit a draft booking. Auto-confirms based on approval levels:
- `none` items: always auto-confirmed
- `low` items: auto-confirmed for project leaders and managers, needs manager approval for regular leaders
- `high` items: auto-confirmed for managers only, needs manager approval for everyone else

If `force_approval` is true, the booking always goes to `submitted` regardless of approval levels — useful when the leader wants a manager to review even freely bookable items.

If any item triggers approval (or `force_approval` is set), the whole booking transitions to `submitted` and a `submitted` booking event is created with the optional message.

**Body**
```json
{
  "message": "Vi behöver detta för hajk, kort varsel",
  "force_approval": false
}
```
All fields optional.

**Response** `200` | `400` | `404`

### 🔒 `POST /api/v0/bookings/{id}/approve`
Approve a submitted booking. Transitions to `confirmed`. Creates an `approved` booking event.

**Body**
```json
{"message": "Godkänt, lycka till!"}
```
`message` is optional.

**Response** `200` | `403` | `404`

### 🔒 `POST /api/v0/bookings/{id}/reject`
Reject a submitted booking. Transitions back to `draft` so the leader can edit and resubmit. Creates a `rejected` booking event.

**Body**
```json
{"message": "Boka färre, vi har inte tillräckligt"}
```
`message` is optional.

**Response** `200` | `403` | `404`

### `GET /api/v0/bookings/{id}/events`
Get the event history for a booking. Returns all booking events ordered chronologically (oldest first).

**Response** `200`
```json
[
  {
    "id": "uuid",
    "booking_id": "uuid",
    "actor_id": "3000005",
    "actor_name": "Hanna Yggdrasil",
    "event_type": "submitted",
    "message": "Vi behöver detta för hajk",
    "metadata": {},
    "created_at": "2026-06-01T10:00:00Z"
  }
]
```

Event types: `submitted`, `approved`, `rejected`, `cancelled`, `note`, `items_changed`, `dates_changed`, `details_changed`.

### `POST /api/v0/bookings/{id}/events`
Add a note to a booking. Any user with access to the booking can add notes regardless of booking status.

**Body**
```json
{"message": "Glömde säga — vi behöver hämta tidigt på morgonen"}
```
`message` is required.

**Response** `201` | `400` | `403` | `404`

### `POST /api/v0/bookings/{id}/cancel`
Cancel a booking. Drafts are deleted entirely (returns 204). Other bookings transition to `cancelled` (returns 200). Cannot cancel returned or already cancelled bookings.

**Response** `200` | `204` | `400` | `403` | `404`

### `POST /api/v0/bookings/{id}/copy`
Create a new draft booking with the same team, notes, and items as the source. Dates are set to today + 7 days as placeholders. Items that no longer exist are silently skipped.

**Response** `201`
```json
{
  "booking": { ... },
  "items_copied": 5,
  "items_total": 5
}
```

### `POST /api/v0/bookings/{id}/pickup`
Transition a confirmed or approved booking to `picked_up`. Saves the current status (`confirmed` or `approved`) as `pre_pickup_status` so it can be restored if all pickups are undone. Access: creator, team members, or equipment manager.

**Response** `200` | `400` | `403` | `404`

### `PUT /api/v0/bookings/{id}/items/{itemId}/pickup`
Set the pickup status for a single booking item. Booking must be in `picked_up` status. Access: creator, team members, or equipment manager. Logs a `picked_up` article event with the acting user.

Sending an empty string clears the pickup status (undo). If all items in the booking have their pickup status cleared, the booking automatically reverts to its pre-pickup status (`confirmed` or `approved`).

Optionally report the article's condition at pickup via `article_status` and `comment`. When `article_status` is set, `comment` is required.
- `reported_usable` with `pickup_status: picked_up` — pick it up but flag the issue
- `reported_unusable` with `pickup_status: lost` — don't pick it up, report as unusable
- `lost` with `pickup_status: lost` — article is missing

**Body**
```json
{"pickup_status": "picked_up", "article_status": "reported_usable", "comment": "Burner is wobbly"}
```
Valid `pickup_status` values: `picked_up`, `lost`, `""` (undo). `article_status` is optional: `reported_usable`, `reported_unusable`, `lost`.

**Response** `200` | `400` | `403` | `404`

### `POST /api/v0/bookings/{id}/items/{itemId}/swap`
Replace the article on a booking item during pickup. The new article must be available for the booking's date range. Sets pickup_status to `swapped`. Booking must be in `picked_up` status. Access: creator, team members, or equipment manager. Logs a `picked_up` article event for the new article.

**Body**
```json
{"new_article_id": "uuid"}
```

**Response** `200` | `400` | `403` | `404` | `409` (article not available)

### `POST /api/v0/bookings/{id}/return`
Transition a picked_up booking to `returned`. All items must have a final return status (not null, not delayed). Access: creator, team members, or equipment manager.

**Response** `200` | `400` | `403` | `404`

### `PUT /api/v0/bookings/{id}/items/{itemId}/return`
Set the return status for a single booking item. Booking must be in `picked_up` status. Access: creator, team members, or equipment manager.

Side effects — article status is orthogonal to booking state. Only explicit condition reports change the article:
- `returned_ok` — no change to article status (condition preserved), logs `returned` event
- `delayed` — no article status change, item stays on loan, logs `returned` event with `delayed` status
- `reported_usable` — sets article status to `reported_usable`, logs `issue_reported` event
- `reported_unusable` — sets article status to `reported_unusable`, logs `issue_reported` event
- `lost` — sets article status to `lost`, logs `issue_reported` event
- `""` (undo) — no change to article status

**Body**
```json
{
  "return_status": "returned_ok",
  "expected_return_date": "2026-06-10",
  "notes": "Optional, used as issue description for broken/lost"
}
```
Valid values: `returned_ok`, `delayed`, `reported_usable`, `reported_unusable`, `lost`, `""` (undo). `expected_return_date` required when status is `delayed`.

**Response** `200` | `400` | `403` | `404`

---

## Teams

Teams represent troops ("Avdelning") and roles ("Roll") — the organizational groups that bookings are made for. Each team has a configurable access level (view, book, trusted, manager). Membership comes from OIDC token claims, mapped via `team_claim_mappings`.

### `GET /api/v0/teams`
List all teams for the group, ordered by name. Includes claim mappings per team.

**Response** `200`
```json
[
  {"id": "uuid", "name": "Yggdrasil", "type": "troop", "access_level": "book", "claim_mappings": [{"claim_scope": "troop", "claim_id": "17443"}], ...},
  {"id": "uuid", "name": "IT-gruppen", "type": "role", "access_level": "manager", "claim_mappings": [{"claim_scope": "group", "claim_id": "it_manager"}], ...}
]
```

### 🔒 `POST /api/v0/teams`
Create a team with optional claim mapping.
```json
{"name": "Yggdrasil", "type": "troop", "access_level": "book", "claim_scope": "troop", "claim_id": "17443"}
```
`type` defaults to `troop`. Valid values: `troop`, `role`.
`access_level` defaults to `book`. Valid values: `view`, `book`, `trusted`, `manager`.
`claim_scope` and `claim_id` are optional — creates a claim mapping if both provided.

**Response** `201` | `400` | `403`

### 🔒 `PUT /api/v0/teams/{id}`
Update a team's name, type, or access level.
```json
{"name": "Yggdrasil", "type": "troop", "access_level": "trusted"}
```
All fields optional — only provided fields are updated.

**Response** `200` | `400` | `403` | `404`

### 🔒 `DELETE /api/v0/teams/{id}`
Delete a team. Blocked if the team has active bookings (409).

**Response** `204` | `403` | `404` | `409`

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
**Response** `204` | `403` | `404` | `409` (`has_articles` with `count`)

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
**Response** `204` | `403` | `404` | `409` (`has_articles` with `count`)

---

## Images

Product images are shared per product type + location (`commercial_name + location_id`). Multiple images per product, stored in `product_images` table with metadata (title, description, format, sharing, attribution). Issue images are standalone, referenced by UUID in article event metadata.

Images are stored as WebP on a Docker volume. Two variants per image: source (2560px longest edge, 2048px for square, q85) and thumbnail (400px height, q75). Thumbnail width varies by format (landscape 533×400, portrait 300×400, square 400×400). On-demand JPEG conversion for download.

### 🔒 `POST /api/v0/images/product`
Upload a product image. Multipart form upload. Accepts JPEG, PNG, WebP, HEIC up to 25MB. Client crops via cropperjs; server validates ratio, strips EXIF, generates source + thumbnail WebP. Appends to `image_ids` on all articles matching commercial_name + location_id. Upload permission controlled by `image_upload_role` group setting.

**Form fields**:
- `file` — image file (pre-cropped by client)
- `commercial_name` — product type (e.g. "Sibley")
- `location_id` — location UUID
- `title` — image title (optional, defaults to empty)
- `description` — image description (optional)
- `format` — `landscape` (4:3), `portrait` (3:4), or `square` (1:1). Default: `landscape`
- `shared` — `true` to share with other scout groups (optional, default false)
- `attribution_mode` — `first_name` (default), `full_name`, or `custom`
- `attribution_name` — free text attribution (only used when mode is `custom`)

**Response** `200`
```json
{"image": {"id": "uuid", "file_id": "uuid", "title": "...", ...}, "image_ids": ["uuid", ...]}
```

### 🔒 `POST /api/v0/images/product/from-shared`
Add a shared image to an article group. Creates a new `product_images` row referencing the same files on disk.

**Body** (JSON):
- `source_image_id` — UUID of the shared image
- `commercial_name` — target product type
- `location_id` — target location UUID
- `title` — title for the new reference
- `description` — description for the new reference

**Response** `200` — same as upload

### `GET /api/v0/images/product`
List product images for an article group.

**Query parameters**: `commercial_name`, `location_id`

**Response** `200` — array of image objects

### `GET /api/v0/images/product/{imageId}`
Get metadata for a single product image, including `ref_count` (how many product_images rows reference the same file).

### `PUT /api/v0/images/product/{imageId}`
Update metadata for a product image. Uploader or equipment manager can edit.

**Body** (JSON):
```json
{
  "title": "Sibley 3",
  "description": "Insida med extra markis",
  "shared": false,
  "attribution": "Anna, Mälarscouterna"
}
```

**Response** `200` | `400` | `403` | `404`

### 🔒🔧 `PUT /api/v0/images/product/reorder`
Reorder images for an article group. Manager only.

**Body** (JSON): `{"commercial_name": "...", "location_id": "...", "image_ids": ["uuid", ...]}`

### 🔒 `DELETE /api/v0/images/product/{imageId}`
Delete a product image. Removes from article `image_ids`, deletes `product_images` row. Files deleted only if no other rows reference the same `file_id`. Uploader or equipment manager can delete.

**Query parameters**: `commercial_name`, `location_id`

**Response** `204`

### `GET /api/v0/images/shared`
Browse shared images across all groups plus own group's images. Attribution resolved per `attribution_mode`.

**Query parameters**: `search` (optional, filters on title/description)

### `GET /api/v0/images/my`
List images uploaded by the current user, with usage counts (`own_group_count`, `other_group_count`).

### `GET /api/v0/images/my/{imageId}/articles`
List article groups using a specific image (deduplicated by commercial_name + location).

### `DELETE /api/v0/images/my/{imageId}`
Delete own image. Removes from all articles' `image_ids` in the group, deletes row, deletes files if no other references.

### `POST /api/v0/images/issue`
Upload an issue report image. Any authenticated user. No crop. Returns UUID for inclusion in issue report metadata.

**Form fields**: `file` — image file

**Response** `200`
```json
{"image_id": "uuid"}
```

### `GET /api/v0/images/{uuid}.webp`
Serve the source image. Returns `image/webp` with immutable cache headers.

### `GET /api/v0/images/{uuid}_thumb.webp`
Serve the thumbnail.

### `GET /api/v0/images/{uuid}.webp?format=jpeg`
Convert and serve as JPEG (quality 85) with `Content-Disposition: attachment` for download.

---

## Group Settings

All endpoints require `equipment_manager` role.

### 🔒 `GET /api/v0/group-settings`
Returns group settings. SMTP key is returned masked.

**Response** `200`
```json
{
  "notification_email_from": "utrustning@example.com",
  "smtp_key_set": true,
  "smtp_key_masked": "sk-...7f2a",
  "gchat_webhook_url": "https://chat.googleapis.com/...",
  "default_approval_level": "none",
  "default_access_unknown": "view",
  "default_access_troop": "book",
  "default_access_role": "book",
  "image_upload_role": "book"
}
```

### 🔒 `PUT /api/v0/group-settings`
```json
{
  "notification_email_from": "utrustning@example.com",
  "smtp_key": "sk-new-key",
  "gchat_webhook_url": "https://chat.googleapis.com/...",
  "default_approval_level": "none",
  "default_access_unknown": "view",
  "default_access_troop": "book",
  "default_access_role": "book",
  "image_upload_role": "book"
}
```
`smtp_key`: `null` = keep existing, `""` = clear, non-empty = encrypt and store.

**Response** `200` | `400` | `403`

---

## Error format

All errors return:
```json
{"error": "error_key"}
```

Error keys are short English strings (e.g. `"invalid id"`, `"article not found"`, `"forbidden"`, `"group_not_found"`). The frontend maps these to translated user-facing messages.

Notable error keys:
- `group_not_found` (403) — the user's group (from JWT claims) doesn't exist in the database. Returned by the user upsert middleware when the group_id FK constraint fails.
- `has_articles` (409) — returned by `DELETE /locations/{id}` and `DELETE /categories/{id}` when articles reference the entity. Response includes `"count": N`.

---

## Authentication

In production: `Authorization: Bearer <jwt>` header with a valid Keycloak token.

In dev mode (`DEV_MODE=true`): `X-Dev-Role-Override: <persona>` header. Available personas:
- `manager-equipment` — manager access via Utrustningsgruppen
- `project-unit-leader` — trusted access via Valborgskommittén + book via Yggdrasil
- `project-leader` — trusted access via Valborgskommittén
- `leader-team-it` — manager access via IT-gruppen + book via Yggdrasil
- `leader-yggdrasil` — book access via Yggdrasil
- `leader-flaskpost` — book access via Flaskpostorné
- `other-kar-leader` — book access in group 999 (Testkåren), team Avdelning 1, for multi-tenancy testing

Also in dev mode: `X-Dev-Claims: <json>` header with a JSON-encoded claims object for testing arbitrary claim combinations (used by integration tests).
