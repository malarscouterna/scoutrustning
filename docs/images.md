# Image Handling

Design doc for image upload, processing, storage, and display. Covers both product images and issue report images.

## Overview

Two types of images:

1. **Product images** — per product type + location (`commercial_name + location_id`). All articles of the same type at the same location share one image. Equipment managers upload. 4:3 crop enforced (later: client-side via cropperjs; initially: server-side center crop).
2. **Issue report images** — attached to article events when reporting issues. Any user uploads. No crop requirement.

## Architecture

```
Browser → SvelteKit proxy → Go API → process (govips) → disk (Docker volume)
                                   ← serve from disk ←
```

- Upload: `POST /api/v0/images/product` (manager) and `POST /api/v0/images/issue` (any user)
- Serve: `GET /api/v0/images/{uuid}.webp` and `GET /api/v0/images/{uuid}_thumb.webp`
- Storage: Docker volume mounted at `/data/images` in the API container
- Processing: govips (libvips CGO wrapper) — handles JPEG, PNG, HEIC input; outputs WebP

## Storage model

No new table. Images are referenced by existing columns:

- **Product images**: `articles.image_path` (text, nullable). All articles sharing `commercial_name + location_id` store the same UUID value. When a manager uploads a product image, all matching articles are updated. The value is just the UUID (e.g. `"a1b2c3d4-..."`), not a full path — the API constructs the file path from it.
- **Issue report images**: `article_events.metadata` jsonb field. The `image_id` key stores the UUID. Events of type `issue_reported` can have an attached image.

Pros of keeping `image_path` on articles (vs separate table):
- No new table, no joins, no migration complexity
- Image UUID comes back in every article query automatically — no extra fetch
- Shared field propagation pattern already exists for articles (description, instructions, etc.)

Cons:
- Denormalized — N articles share the same value, must stay in sync
- Upload/delete must update all matching articles (one UPDATE query, trivial)
- Risk of drift if a bug updates only one article — mitigated by always using a bulk UPDATE scoped to `commercial_name + location_id + group_id`

## Processing pipeline

Using `github.com/davidbyttow/govips/v2`:

1. Accept upload (multipart form, max 25MB)
2. Validate MIME type (JPEG, PNG, HEIC/HEIF)
3. Load with govips (auto-detects format, applies EXIF rotation)
4. Strip all EXIF/metadata
5. For product images: center-crop to 4:3 aspect ratio (later: client sends crop coordinates)
6. Generate two variants:
   - **Source**: longest edge 1920px, WebP quality 80
   - **Thumbnail**: 400×300px, WebP quality 70
7. Save as `{uuid}.webp` and `{uuid}_thumb.webp`
8. Update database references
9. Return the UUID

## API endpoints

### Upload product image

```
POST /api/v0/images/product
Content-Type: multipart/form-data

Fields:
  file: <image file>
  commercial_name: "Sibley"
  location_id: "uuid"

Response: 200
{
  "image_id": "a1b2c3d4-..."
}
```

Requires `equipment_manager` role. Updates `image_path` on all articles matching `commercial_name + location_id + group_id`.

If articles already have an `image_path`, the old files are deleted from disk.

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

Any authenticated user. Returns the UUID. The caller then includes `image_id` in the issue report request body. The image is stored immediately but only referenced when the article event is created.

Orphaned images (uploaded but never referenced) can be cleaned up by a background job later.

### Serve image

```
GET /api/v0/images/{uuid}.webp        → source (1920px)
GET /api/v0/images/{uuid}_thumb.webp  → thumbnail (400×300)
```

Public within the app (requires auth via proxy). Returns `image/webp` with `Cache-Control: public, max-age=31536000, immutable` (images are content-addressed by UUID — new upload = new UUID).

Optional query parameter `?format=jpeg` converts on the fly to JPEG (quality 85) for download/sharing. JPEG is universally supported on all devices and apps. The UI offers a "Ladda ner" (download) link that uses this parameter with a `Content-Disposition: attachment` header.

### Delete product image

```
DELETE /api/v0/images/product?commercial_name=Sibley&location_id=uuid
```

Requires `equipment_manager` role. Clears `image_path` on all matching articles, deletes files from disk.

## Testing

Image processing requires libvips. Install on the dev machine:

```bash
sudo apt install libvips-dev   # Ubuntu/Debian
```

This lets `go test ./...` work directly, keeping the existing testcontainers workflow unchanged. CI builds inside Docker where libvips is already present.

Integration tests for images:
- Upload a product image (JPEG), verify files on disk, verify `image_path` set on articles, serve back as WebP, serve as JPEG via `?format=jpeg`, delete and verify cleanup.
- Upload an issue image, verify files on disk, verify serve works.
- Access control: leader gets 403 on product upload, can upload issue images.
- Replace: upload new image for same group, verify old files deleted.

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

Add image volume to the API service:

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

The dev override mounts a local directory instead for easy inspection:

```yaml
# docker-compose.override.yml
services:
  api:
    volumes:
      - ./data/images:/data/images
```

## Implementation steps

### Step 1: Infrastructure — govips + image processing + serving

Backend only. No frontend changes.

- [ ] Add `govips` dependency to `go.mod`
- [ ] Update API Dockerfile (dev + build + production stages) with libvips
- [ ] Add `images` volume to `docker-compose.yml` and override
- [ ] Create `api/internal/images/` package:
  - `process.go` — load image (JPEG/PNG/HEIC), strip EXIF, auto-rotate, resize, crop, encode WebP, save to disk
  - `handler.go` — HTTP handlers for upload and serve
- [ ] Add `IMAGE_DIR` env var to server startup, pass to handler
- [ ] Wire routes: `POST /api/v0/images/product`, `GET /api/v0/images/{uuid}.webp`, `GET /api/v0/images/{uuid}_thumb.webp`, `DELETE /api/v0/images/product`
- [ ] Add sqlc query: `UpdateArticleGroupImagePath` — bulk update `image_path` for all articles matching `commercial_name + location_id + group_id`
- [ ] Add sqlc query: `ClearArticleGroupImagePath` — clear `image_path` for matching articles
- [ ] Integration test: upload product image, verify files on disk, verify `image_path` set on articles, serve image back, delete image

Deliverable: can upload/serve/delete product images via curl. No UI yet.

### Step 2: Product images in browse + article detail

Frontend displays product images where they exist.

- [ ] Update `Article` interface in `client.ts` to include `image_path`
- [ ] Browse page: show thumbnail in group header row (small, left of commercial name)
- [ ] Article detail page: show source image at top of page
- [ ] Placeholder/icon when no image exists
- [ ] Lazy loading (`loading="lazy"`) on thumbnails in browse list

Deliverable: uploaded images visible in the UI. No upload UI yet.

### Step 3: Product image upload UI

Equipment managers can upload images from the article edit page and browse page.

- [ ] Add `uploadProductImage` and `deleteProductImage` to API client
- [ ] Article edit page (individually tracked): image upload area with preview, replace, delete
- [ ] Article edit page (quantity tracked / group edit): same, applies to the group
- [ ] Browse page (manager mode): upload button on expanded group (when no image), replace/delete when image exists
- [ ] Show upload progress indicator
- [ ] Validate file type client-side before upload (accept="image/jpeg,image/png,image/heic")
- [ ] Max file size warning client-side (25MB)

Deliverable: managers can upload product images from the UI.

### Step 4: Issue report images

Any user can attach an image when reporting an issue.

- [ ] Add `POST /api/v0/images/issue` endpoint
- [ ] Update `ReportIssueForm` component: optional image attachment
- [ ] Update `UpdateStatus` handler to accept `image_id` in request body, store in event metadata
- [ ] Article event history: display issue images inline (thumbnail, click for full size)
- [ ] Issues page: show thumbnail in issue list when image exists
- [ ] Integration test: report issue with image, verify event metadata, verify image served

Deliverable: users can attach photos when reporting issues.

### Step 5: Client-side crop for product images

Enforce 4:3 aspect ratio via interactive crop UI before upload.

- [ ] Add `cropperjs` dependency to web
- [ ] Create `ImageCropModal` component: loads image, shows crop overlay locked to 4:3, outputs cropped blob
- [ ] Wire into product image upload flow: select file → crop modal → upload cropped result
- [ ] Remove server-side center-crop fallback (or keep as safety net)

Deliverable: consistent 4:3 product images via interactive crop.

### Step 6: Image in booking views

Show product thumbnails in booking-related pages.

- [ ] Booking detail: show thumbnail next to each item in the checklist
- [ ] Pickup checklist: thumbnail helps identify the right item
- [ ] Availability/booking page: thumbnail in the product list
- [ ] Book page: thumbnail in cart items

Deliverable: images visible throughout the booking flow.

## File size estimates

| Variant | Dimensions | Quality | Typical size |
|---|---|---|---|
| Source | 1920px longest edge | WebP q80 | 200KB–1MB |
| Thumbnail | 400×300px | WebP q70 | 10–30KB |

At ~200 unique product types per group: ~200MB source + ~6MB thumbnails per group. Well within Docker volume limits.

## Security considerations

- File type validated server-side (govips rejects non-image input)
- Max upload size enforced (25MB raw, `http.MaxBytesReader`)
- EXIF stripped completely (no GPS, device info, or PII leaks)
- Images served with immutable cache headers (UUID-based, no guessing)
- Auth required for all image endpoints (via existing middleware)
- Product upload restricted to equipment managers
- No directory traversal — UUID is validated, path is constructed server-side

## Decisions and trade-offs

| Decision | Rationale |
|---|---|
| govips (CGO) over pure Go | Only option for HEIC + WebP. Adds ~45MB to Docker image, ~1s to rebuilds. Acceptable. |
| WebP output | ~3x smaller than JPEG at similar quality. All modern browsers support it. |
| UUID-based filenames | Content-addressed, cache-friendly, no conflicts, no path traversal risk. |
| No separate images table | `image_path` on articles + `metadata` on events is sufficient. Avoids new table + joins. |
| Server-side center crop initially | Simpler first step. Client-side cropperjs added in step 5. |
| Two variants only | Source + thumbnail covers all current UI needs. Add more sizes if needed later. |
| Orphan cleanup deferred | Uploaded-but-unreferenced issue images are rare and small. Background cleanup can be added later. |
| WebP storage + JPEG download | Store WebP (3x smaller), serve WebP by default (all modern browsers), offer JPEG on-demand for download/sharing. No need to store JPEG variants — conversion is fast and downloads are infrequent. |
