# Image Handling

Design doc for image upload, processing, storage, and display. Covers both product images and issue report images.

## Overview

Two types of images:

1. **Product images** — per product type + location (`commercial_name + location_id`). All articles of the same type at the same location share images. Client-side crop enforced (cropperjs) with selectable format: landscape (4:3), portrait (3:4), or square (1:1). Images can be shared with other scout groups.
2. **Issue report images** — attached to article events when reporting issues. Any user uploads. No crop requirement.

**Important**: Images are compressed for web display. Originals are not saved on the server — only the processed WebP variants are stored. The UI makes this clear during upload.

## Architecture

```
Browser → cropperjs (crop) → SvelteKit proxy → Go API → process (govips) → disk (Docker volume)
                                                       ← serve from disk ←
```

- Upload: `POST /api/v0/images/product` and `POST /api/v0/images/issue`
- Serve: `GET /api/v0/images/{uuid}.webp` and `GET /api/v0/images/{uuid}_thumb.webp`
- Storage: Docker volume mounted at `/data/images` in the API container
- Processing: govips (libvips CGO wrapper) — handles JPEG, PNG, HEIC input; outputs WebP

## Storage model

### `product_images` table

```sql
product_images (
  id               uuid PK DEFAULT gen_random_uuid(),
  file_id          uuid NOT NULL,  -- references files on disk ({file_id}.webp / {file_id}_thumb.webp)
  group_id         text NOT NULL FK → groups,
  uploaded_by       text NOT NULL FK → users,  -- member ID, for audit
  title            text NOT NULL DEFAULT '',
  description      text NOT NULL DEFAULT '',
  format           text NOT NULL DEFAULT 'landscape', -- 'landscape' (4:3), 'portrait' (3:4), 'square' (1:1)
  shared           boolean NOT NULL DEFAULT false,
  attribution      text NOT NULL DEFAULT '',           -- resolved display string, e.g. "Anna, Mälarscouterna"
  created_at       timestamptz NOT NULL DEFAULT now()
)
```

`file_id` is separate from the row `id` because shared images create new rows pointing to the same files on disk. Row `id` is unique per group reference; `file_id` identifies the physical files.

### Attribution

`uploaded_by` stores the member ID (FK to users) for audit. The `attribution` column stores the resolved display string for photographer credit when the image is shared. The client builds this string from one of three modes in the upload UI:

| Mode | Example result |
|---|---|
| First name + group (default) | "Anna, Mälarscouterna" |
| Full name + group | "Anna Svensson, Mälarscouterna" |
| Custom free text | "Foto: Scouternas Folkhögskola" |

The API stores and returns only the final string — the mode is a UI-only concept.

### Article references

`articles.image_ids` (jsonb array of UUID strings) references `product_images.id`. All articles sharing `commercial_name + location_id` store the same array. This is the existing model from migration 00013.

When a product image is added to an article group, the `product_images` record stores the canonical title and description set by the uploader. The `image_ids` array on articles just references these — no per-article override of metadata. When browsing shared images and adding one to a different article, the title is set to match the new article's name, and the description is pre-populated from the original but editable before saving. This creates a new `product_images` row referencing the same files on disk — see "Shared images" below.

### Issue report images

`article_events.metadata` jsonb field. The `image_id` key stores the UUID. Events of type `issue_reported` can have an attached image. Issue images don't use the `product_images` table — they're standalone files on disk.

### Why a table instead of jsonb metadata

The original plan stored images as bare UUIDs with no metadata. The new requirements (title, description, sharing, uploader attribution, format) need structured per-image data that:
- Is queryable (browse shared images across groups)
- Has FK integrity (uploader → users)
- Supports cross-group references

A table is the right fit. The `image_ids` jsonb array on articles stays as the lightweight reference.

## Shared images

When group B adds a shared image from group A to one of their articles:
- A **new `product_images` row** is created in group B's scope, with a new title (matching group B's article name) and description (pre-populated from group A's original, editable)
- The new row stores the **same `file_id`** as the original — both point to the same files on disk
- The new row has its own `shared` flag (defaults to false — group B can choose to share it further)

**Deletion**: If group A deletes the original image, the files are removed from disk only if no other `product_images` rows reference the same `file_id`. If other rows exist, only the DB row is deleted. If the files are gone, the serve endpoint returns 404 and the frontend shows a placeholder.

**Browse shared images**: `SELECT FROM product_images WHERE shared = true` (cross-group) plus `WHERE group_id = @group_id` (own group). Returns image metadata + resolved attribution string. A "potential match" indicator highlights images whose title or description matches the current article's commercial name.

## Processing pipeline

Using `github.com/davidbyttow/govips/v2`:

1. Accept upload (multipart form, max 25MB)
2. Validate MIME type (JPEG, PNG, HEIC/HEIF, WebP)
3. Load with govips (auto-detects format, applies EXIF rotation)
4. Strip all EXIF/metadata
5. For product images: crop to the format specified by the client (the client sends pre-cropped data via cropperjs, but the server validates the ratio and trims if needed)
6. Generate two variants:
   - **Source**: longest edge 2560px (2048px for square), WebP quality 85
   - **Thumbnail**: 400px height (width varies by format), WebP quality 75
7. Save as `{file_id}.webp` and `{file_id}_thumb.webp`
8. Insert `product_images` row, update `image_ids` on articles
9. Return the image metadata

### Thumbnail dimensions

Thumbnails are saved at **400px height**. Width varies by format:

| Format    | Aspect | Thumb size | CSS display (phone) | CSS display (tablet+) |
|-----------|--------|------------|---------------------|-----------------------|
| landscape | 4:3    | 533×400    | ~300×225            | ~400×300              |
| portrait  | 3:4    | 300×400    | ~169×225            | ~225×300              |
| square    | 1:1    | 400×400    | ~225×225            | ~300×300              |

Saving at 400px height gives headroom for larger displays without visible quality loss. The CSS constrains display size.

If display sizes change later, thumbnails can be regenerated from the source files (2560px) which are always kept.

## Client-side crop (cropperjs)

Product image uploads use [cropperjs](https://github.com/fengyuanchen/cropperjs) for client-side cropping before upload:

1. User selects a file
2. Crop UI appears with a single format switcher: **Liggande** (landscape 4:3), **Stående** (portrait 3:4), **Kvadrat** (square 1:1)
3. User adjusts the crop area
4. On confirm, the cropped image is sent to the server as a blob
5. Server validates the ratio (within tolerance) and processes normally

The format switcher determines both aspect ratio and orientation in one choice — no separate toggles. This ensures consistent layouts across the app and gives users control over framing. The server still does the final resize/encode — cropperjs just handles the crop selection.

For issue report images, no crop UI — images upload as-is.

## Upload UX

### Product image upload

On the article edit page, images are shown in a horizontal scrollable row with titles. Two buttons:
- **Ladda upp** — opens file picker → cropperjs crop UI → upload
- **Bläddra** — opens shared image browser (see below)

On upload:
- **Title**: auto-set to `{commercial_name} {index}` (e.g. "Sibley 3" for the third image). Editable.
- **Description**: empty by default. Editable.
- **Share checkbox**: unchecked by default. Checking it reveals:
  - Conditions text: "Du behöver ha tagit bilden själv eller ha tillåtelse. Alla identifierbara personer måste ha gett sitt godkännande. Bilder delas enbart bakom scoutinloggning."
  - **Attribution** radio buttons: "Förnamn, kårnamn" (default), "Fullständigt namn, kårnamn", "Egen text" (reveals free text input)
  - Preview box showing how the attribution will appear
- **Note**: "Bilder komprimeras för webben. Originalet sparas inte."

### Upload permission

Who can upload product images is controlled by a group setting: `image_upload_role` on `group_settings`.

| Setting value | Who can upload | Swedish label |
|---|---|---|
| `any` | Everyone including view-only | Alla inloggade |
| `leader` | Leaders + project leaders + managers | Alla som kan boka |
| `project_leader` | Project leaders + managers | Projektledare och utrustningsansvariga |
| `equipment_manager` | Managers only | Bara utrustningsansvariga |

Default: `leader` — any user who can book can also upload images. View-only users (authenticated but no leader role) can browse images but not upload.

The column is `image_upload_role` on `group_settings` (not `_level` — this is a role threshold, not an approval level). This avoids conflating with the article `approval_level` system (`none`/`low`/`high`) which describes how much approval an article requires, not who can perform an action.

The setting appears on the group settings page ("Gruppinställningar") as a select: "Vem kan ladda upp bilder?" with options matching the Swedish labels above.

The upload endpoint checks this setting and returns 403 if the user's role doesn't meet the threshold. Equipment managers always have upload access regardless of the setting.

### Shared image browser

A modal/page where users can browse available images:
- Shows images from own group + images shared by other groups
- Each image shows: thumbnail, title, description preview, uploader name/group (per toggles)
- Search/filter by title/description
- **Potential match indicator**: if a shared image's title or description contains the current article's commercial name, highlight it (e.g. badge or sort to top)
- Selecting an image:
  - Title is set to match the current article's commercial name + next index
  - Description is pre-populated from the original image's description
  - Both are editable before confirming
  - Creates a new `product_images` row in the current group referencing the same file_id

### Issue image upload

Simple file picker, no crop, no metadata. Returns UUID for attachment to article event.

## Display contexts

### Browse page (all users)

When expanding an article group, images show immediately (no button press needed). Thumbnails at 225px height (phones) / 300px height (tablets+). Description shown below each thumbnail, **2 lines max** via CSS `line-clamp-2` — consistent across screen sizes regardless of text length. Tap thumbnail → fullscreen (PhotoSwipe) showing title + full description.

**Not shown in manage mode** — managers see the edit controls instead.

### Article detail page

Thumbnails shown at same size as browse. Horizontal scroll if multiple. Title shown below each. Tap → fullscreen with title + description.

Users with upload permission see an upload button here (not just on the edit page). They can add images but **cannot delete other users' images or reorder** — only their own images can be deleted. Managers can delete any image and reorder.

### Article edit page (manager)

Images in a horizontal scrollable row. Each shows thumbnail + title. Delete button (×) on each — removes from this article group, does not delete the `product_images` row or files (other groups may reference it). Two action buttons: "Ladda upp" and "Bläddra".

### Booking detail / pickup / return

Images show when pressing on an article card (expanding it). Thumbnail + description (2 lines, `line-clamp-2`). Tap → fullscreen.

### Fullscreen (PhotoSwipe)

Shows source image (2560px, 2048px for square). Title displayed as caption. Full description below title.

## API endpoints

### Upload product image

```
POST /api/v0/images/product
Content-Type: multipart/form-data

Fields:
  file: <cropped image file>
  commercial_name: "Sibley"
  location_id: "uuid"
  title: "Sibley 3"
  description: "Insida med extra markis"
  format: "landscape"
  shared: "false"
  attribution: "Anna, Mälarscouterna"

Response: 200
{
  "image": {
    "id": "a1b2c3d4-...",
    "file_id": "a1b2c3d4-...",
    "title": "Sibley 3",
    "description": "Insida med extra markis",
    "format": "landscape",
    "shared": false
  },
  "image_ids": ["uuid1", "uuid2", "a1b2c3d4-..."]
}
```

Requires upload permission (see "Upload permission" above). Creates `product_images` row, appends to `image_ids` on all articles matching `commercial_name + location_id + group_id`.

### Add shared image to article group

```
POST /api/v0/images/product/from-shared
Content-Type: application/json

{
  "source_image_id": "uuid-of-shared-image",
  "commercial_name": "Sibley",
  "location_id": "uuid",
  "title": "Sibley 3",
  "description": "Insida med extra markis"
}

Response: 200
{
  "image": { ... },
  "image_ids": [...]
}
```

Creates a new `product_images` row in the current group with the same `file_id`. Does not copy files.

### Browse shared images

```
GET /api/v0/images/shared?search=sibley

Response: 200
[
  {
    "id": "uuid",
    "file_id": "uuid",
    "title": "Sibley tält",
    "description": "Utsida i solljus",
    "format": "landscape",
    "attribution": "Anna, Mälarscouterna",
    "created_at": "2025-01-15T10:00:00Z"
  }
]
```

Returns images where `shared = true` (all groups) plus all images from the current group (regardless of shared flag).

### Get image metadata

```
GET /api/v0/images/product/{imageId}

Response: 200
{
  "id": "uuid",
  "file_id": "uuid",
  "title": "Sibley 3",
  "description": "Insida med extra markis",
  "format": "landscape",
  "shared": false,
  "attribution": "Anna Svensson, Mälarscouterna",
  "ref_count": 2,
  "created_at": "2025-01-15T10:00:00Z"
}
```

### List image metadata for article group

```
GET /api/v0/images/product?commercial_name=Sibley&location_id=uuid

Response: 200
[
  { "id": "uuid1", "title": "Sibley 1", "description": "...", ... },
  { "id": "uuid2", "title": "Sibley 2", "description": "...", ... }
]
```

Returns metadata for all images in the article group's `image_ids` array, in order.

### Upload issue image

```
POST /api/v0/images/issue
Content-Type: multipart/form-data

Fields:
  file: <image file>

Response: 200
{
  "image_id": "a1b2c3d4-..."
}
```

Any authenticated user. No crop, no metadata table entry.

### Serve image

```
GET /api/v0/images/{uuid}.webp        → source (2560px)
GET /api/v0/images/{uuid}_thumb.webp  → thumbnail (400px height)
```

Returns `image/webp` with `Cache-Control: public, max-age=31536000, immutable` (images are content-addressed by UUID — new upload = new UUID).

Optional `?format=jpeg` converts on the fly to JPEG (quality 85) for download. The UI offers a "Ladda ner" link using this.

### Delete product image

```
DELETE /api/v0/images/product/{imageId}?commercial_name=Sibley&location_id=uuid
```

Requires ownership (uploader) or `equipment_manager` role. Non-uploaders who aren't managers get 403. Removes from `image_ids` on matching articles. Deletes the `product_images` row. Deletes files from disk **only if no other `product_images` rows reference the same `file_id`**.

### Reorder product images

```
PUT /api/v0/images/product/reorder
Content-Type: application/json

{
  "commercial_name": "Sibley",
  "location_id": "uuid",
  "image_ids": ["uuid2", "uuid1", "uuid3"]
}
```

Validates same set of IDs, updates order in `image_ids` array.

## Cache control

Images use **immutable caching**: `Cache-Control: public, max-age=31536000, immutable`. Since each image has a unique UUID and UUIDs never change (new upload = new UUID, edit = new row), browsers cache forever and never re-validate. This is optimal for both performance and bandwidth.

The SvelteKit proxy passes through these headers. No additional cache layer needed.

For broken references (shared image deleted by original group), the serve endpoint returns 404. The frontend shows a placeholder and the broken reference can be cleaned up by the manager.

## Testing

Image processing requires libvips. Install on the dev machine:

```bash
sudo apt install libvips-dev   # Ubuntu/Debian
```

Integration tests for images:
- Upload a product image (JPEG) with metadata, verify files on disk, verify `product_images` row, verify `image_ids` on articles, serve back as WebP, serve as JPEG via `?format=jpeg`, delete and verify cleanup
- Upload with sharing enabled, browse shared from another group, verify attribution resolved correctly
- Add shared image to different article group, verify new row created, same files referenced
- Delete shared image, verify files deleted only when no other rows reference same file_id
- Format variants (landscape, portrait, square)
- Access control: upload permission based on `image_upload_role` setting
- Browse shared: own group images always visible, other groups' shared images visible, non-shared hidden

## Docker changes

### API Dockerfile

```dockerfile
# Dev stage
FROM golang:1.26-alpine AS dev
RUN apk add --no-cache gcc musl-dev vips-dev
RUN go install github.com/air-verse/air@latest
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
ENV MIGRATIONS_DIR=/src/migrations
EXPOSE 8080
CMD ["air"]

# Build stage
FROM golang:1.26-alpine AS build
RUN apk add --no-cache gcc musl-dev vips-dev
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 go build -o /bin/server ./cmd/server

# Production stage
FROM alpine:3.22 AS production
RUN apk add --no-cache ca-certificates tzdata vips
COPY --from=build /bin/server /bin/server
COPY --from=build /src/migrations /migrations
ENV MIGRATIONS_DIR=/migrations
EXPOSE 8080
ENTRYPOINT ["/bin/server"]
```

### docker-compose.yml

```yaml
services:
  api:
    volumes:
      - ./role-mapping.json:/config/role-mapping.json:ro
      - ./dev-personas.json:/config/dev-personas.json:ro
      - images:/data/images
    environment:
      IMAGE_DIR: /data/images

volumes:
  pgdata:
  images:
```

Dev override mounts a local directory:

```yaml
# docker-compose.override.yml
services:
  api:
    volumes:
      - ./data/images:/data/images
```

## File size estimates

| Variant | Dimensions | Quality | Typical size |
|---|---|---|---|
| Source (landscape) | 2560×1920 | WebP q85 | 400KB–1MB |
| Source (portrait) | 1920×2560 | WebP q85 | 400KB–1MB |
| Source (square) | 2048×2048 | WebP q85 | 400KB–1MB |
| Thumbnail | 400px height | WebP q75 | 25–50KB |

At ~200 unique product types per group: ~200MB source + ~6MB thumbnails per group. Well within Docker volume limits.

## Security considerations

- File type validated server-side (govips rejects non-image input)
- Max upload size enforced (25MB raw, `http.MaxBytesReader`)
- EXIF stripped completely (no GPS, device info, or PII leaks)
- Images served with immutable cache headers (UUID-based, no guessing)
- Auth required for all image endpoints (via existing middleware)
- Product upload restricted by `image_upload_role` group setting
- No directory traversal — UUID is validated, path is constructed server-side

## Decisions and trade-offs

| Decision | Rationale |
|---|---|
| govips (CGO) over pure Go | Only option for HEIC + WebP. Adds ~45MB to Docker image, ~1s to rebuilds. Acceptable. |
| WebP output | ~3x smaller than JPEG at similar quality. All modern browsers support it. |
| UUID-based filenames | Content-addressed, cache-friendly, no conflicts, no path traversal risk. |
| `product_images` table | Metadata (title, description, sharing, attribution) needs structured queryable storage with FK integrity. `image_ids` jsonb on articles stays as lightweight reference. |
| `file_id` separate from row `id` | Shared images create new rows pointing to the same files on disk. Row ID is unique per group reference, file ID identifies the physical files. |
| Single `format` field | Combines aspect ratio and orientation into one choice (landscape/portrait/square). Simpler than separate fields, maps 1:1 to the crop UI switcher. |
| Two variants only | Source + thumbnail covers all current UI needs. Add more sizes if needed later. |
| Orphan cleanup deferred | Uploaded-but-unreferenced issue images are rare and small. Background cleanup can be added later. |
| WebP storage + JPEG download | Store WebP (3x smaller), serve WebP by default, offer JPEG on-demand for download. Conversion is fast and downloads are infrequent. |
| Shared images referenced, not copied | Simpler than reference counting. If original is deleted, dependents get 404 — acceptable trade-off for a convenience feature. |

## Implementation steps

### Completed: Infrastructure + basic display + multi-image

- [x] govips dependency, API Dockerfile with libvips, `images` Docker volume
- [x] `api/internal/images/` package: process.go (JPEG/PNG/WebP/HEIC → WebP, EXIF strip, auto-rotate, 4:3 center crop, source + thumbnail variants), handler.go (upload, serve, delete)
- [x] Byte-level MIME detection including HEIC ftyp box and WebP RIFF header sniffing
- [x] On-demand JPEG conversion for download (`?format=jpeg`)
- [x] Product image upload (manager-only), issue image upload (any user), serve with immutable caching
- [x] Migration 00013: `image_ids` jsonb array on articles (replacing single `image_path`)
- [x] Multi-image support: upload appends, reorder endpoint, delete single image
- [x] Frontend: thumbnails in browse page expanded info section and article detail page
- [x] Frontend: PhotoSwipe lightbox viewer (tap to view full size, download as JPEG)
- [x] Frontend: `ImageUpload.svelte` component (manager, article edit page)
- [x] Frontend: `ImageViewer.svelte` component (browse + article detail)
- [x] Seed script uploads images from `docs/seed-images/` directory
- [x] Integration tests: 8 subtests covering upload, serve, replace, delete, access control, JPEG download

### Step 1: `product_images` table + metadata + sharing + upload permission ✅

- [x] Migration 00014: create `product_images` table with `file_id`, add `image_upload_role` to `group_settings` (default `'leader'`), migrate existing `image_ids` UUIDs into rows
- [x] sqlc queries: insert, get by ID, list by IDs, list shared, delete, count by file_id, get upload role, list by uploader, list articles using image, remove image from all articles
- [x] Update image handler to create `product_images` rows on upload (accept title, description, format, shared, attribution)
- [x] Upload permission check: read `image_upload_role` from group settings, compare against user's roles, 403 if insufficient
- [x] Update delete handler to check for other rows referencing same file_id before deleting files
- [x] New endpoints: `GET /images/shared`, `POST /images/product/from-shared`, `GET /images/product`, `GET /images/product/{id}`, `GET /images/my`, `GET /images/my/{id}/articles`, `DELETE /images/my/{id}`
- [x] Update group settings endpoint + frontend to include `image_upload_role`
- [x] Integration tests updated

### Step 2: Format support (landscape / portrait / square) ✅

- [x] `ProcessProductImage` accepts format parameter, crops to matching aspect ratio
- [x] Thumbnail generation: 400px height, width varies by format
- [x] Server-side validation: verify uploaded image roughly matches declared format

### Step 3: Client-side crop UI (cropperjs) ✅

- [x] cropperjs dependency added to web
- [x] `ImageCropDialog` component: file input → crop preview with format switcher (Liggande / Stående / Kvadrat) → crop → return blob
- [x] `ImageUploadDialog` component: wraps crop dialog + metadata fields (title, description, share checkbox with conditions + attribution radio buttons) + upload button
- [x] Note in dialog: "Bilder komprimeras för webben. Originalet sparas inte."
- [x] `ImageUpload.svelte` uses the dialog flow

### Step 3.5: Attribution model ✅

- [x] Migration 00015: replace `name_display` with single `attribution` text column (resolved display string)
- [x] Client builds attribution string from three UI modes (first name + group, full name + group, custom free text)
- [x] Upload dialog: three radio buttons with preview, sends resolved string to API
- [x] "Mina bilder" section on profile page with per-image details, article links, delete

### Step 4: Shared image browser ✅

- [x] Create `SharedImageBrowser` component: modal with search, grid of thumbnails with metadata, potential match indicator
- [x] On select: pre-populate title (article name + index) and description (from original), editable before confirming
- [x] Wire into article edit page as "Bläddra" button
- [x] Deduplicate shared images by `file_id` (DISTINCT ON in SQL)

### Step 5: Display in browse + booking flows (partial)

- [x] PhotoSwipe fullscreen: title + description + attribution as caption overlay
- [x] Correct PhotoSwipe dimensions via `data-pswp-width`/`data-pswp-height` from format metadata
- [ ] Browse page: description preview (2 lines, `line-clamp-2`) below thumbnails
- [ ] Booking detail / pickup / return: images in expanded article card with description preview
- [ ] Handle broken references (404 from deleted shared images) with placeholder

### Step 6: Article edit page image management (partial)

- [x] Horizontal scrollable row of thumbnails on edit page (ImageViewer with showMeta)
- [x] "Ladda upp" button → ImageUploadDialog
- [x] "Bläddra" button → SharedImageBrowser with potential match indicator
- [x] Edit image metadata (title, description, attribution, shared) — full-width inline form, permission-aware (manager: any, user: own)
- [x] Edit from article detail page and profile "Mina bilder"
- [ ] Delete button per image (removes from article group, checks file reference count)
- [ ] Reorder via drag or move buttons

### Step 7: Issue report images

- [ ] Update `ReportIssueForm` component: optional image attachment
- [ ] Update `UpdateStatus` handler to accept `image_id` in request body, store in event metadata
- [ ] Article event history: display issue images inline (thumbnail, click for full size)
- [ ] Issues page: show thumbnail in issue list when image exists
- [ ] Integration test: report issue with image, verify event metadata, verify image served
