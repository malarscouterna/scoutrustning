# Article links

## Why

Articles link out to knowledge that does not belong in the free-text
`instructions` field: user guides, repair manuals, videos, spare-part pages. The
links need a short label, a visibility rule (everyone vs trusted), and routine
checking so the inventory does not accumulate dead links.

## Model

`article_links` (migration `00025_article_links.sql`):

| Column | Purpose |
| --- | --- |
| `group_id` | Tenant. |
| `scope_key` | Product type: `commercial_name`, or `common_name` when the commercial name is empty. |
| `label` | Short description shown in the UI ("Användarinstruktion", "Reparationsmanual", …). Free text, 1–120 chars. |
| `url` | http(s) only, max 2048 chars. Unique per `(group_id, scope_key, url)`. |
| `visibility` | `public` (all group members) or `trusted` (trusted + manager). |
| `created_by` | User who first added the link (kept across edits). |
| checker columns | `last_checked_at`, `last_status` (`unchecked`/`ok`/`broken`), `last_http_status`, `last_error`, `failure_kind`, `acknowledged_*`. |

### Scope, not article id

Links follow the same product-type semantics as `description` and `instructions`:
one set per `commercial_name` (falling back to `common_name`), shared by every
physical copy and every location. This is why the table does not reference
`articles.id`:

- Creating another copy adds the supplied links (upsert) instead of replacing the
  set, so a create can never wipe a product type's shared links.
- Saving an **existing** article with a `links` array replaces that scope's set;
  rows whose URL survives keep their checker state and acknowledgement.
- Renames are conservative (`applyLinkScopeChange`):
  - If another copy still uses the old name, the old product type keeps its links
    and the renamed copy uses the new type's links (possibly none).
  - Renaming the last copy to a new name moves the links.
  - Renaming the last copy into an existing product keeps the target's links and
    removes the old set (a deliberate merge).
  - Link edits submitted in the same save as a rename are ignored; edit links in a
    second save.
- Deleting the last copy of a product type deletes its links
  (`DeleteOrphanedArticleLinks`).

## Visibility and permissions

- Adding/editing/deleting links uses the existing configurable
  `article_edit_role` (default `manager`, minimum `book`), exposed through the
  article routes (`/{id}/links`).
- `trusted` links are filtered out of article list/get responses, availability
  groups and booking items unless `claims.MaxAccess` is `trusted` or `manager`.
- Editors fetch the full set through `GET /articles/{id}/links` so the edit form
  can round-trip links the current viewer cannot see; saving writes the full set
  back. This prevents a book-level editor from silently deleting trusted links.
- The broken-links section lives in the manager-only Kårinställningar tab.

## Checking

`internal/linkcheck` builds a hardened `http.Client`:

- http/https only, no credentials in URLs, 15s timeout, at most 3 redirects.
- Real destination addresses are resolved and rejected when loopback, private,
  link-local, CGNAT, multicast or unspecified (SSRF protection). Proxy env vars are
  ignored so a private proxy cannot bypass the checks. `NewForTest()` allows
  loopback so tests can use `httptest`.
- One `GET`, reading at most 64 KB of the body before closing.

Results are classified as `ok`, `http:<code>`, `timeout`, `dns`, `tls`,
`connection`, `blocked` or `invalid`. Any non-2xx counts as broken; 403/429 are
included because bot protection is common — managers can approve them.

A canceled run (server shutdown, client disconnect, or the "Check now" deadline)
never writes a result: only a genuinely completed request or the per-check 20s
timeout is persisted. This prevents deploys and aborted runs from marking
in-flight links broken.

### Scheduling

- `main.go` runs `linkcheck.RunDue` on startup and every 24h (every 10 minutes in
  genuine dev mode). It checks links older than the interval, max 200 per run.
- Managers can trigger `POST /group-settings/check-links` ("Check now"). It
  checks oldest-checked first (max 200) and returns `checked`, `broken`, `total`
  and `truncated`, so the UI can say when a run was cut short.

### "Mark as working"

`POST /group-settings/broken-links/{id}/ack` stores the link's current
`failure_kind` as `acknowledged_kind`. Acknowledged links stay in the list with
`acknowledged: true` (shown muted, with an **Ångra** / Undo action backed by
`DELETE .../ack`). They are treated as active again if the failure kind changes
(403 → 404, timeout → DNS, …); a successful check clears both status and
acknowledgement.

## UI

- `ArticleLinks.svelte` renders the list below instructions on browse, item page,
  availability picker, booking items and the return checklist.
- Availability groups are keyed by `commercial_name || common_name` (the same
  scope as links), so products without a commercial name get per-item groups and
  the correct links instead of merging into one unnamed group per location.
- `ArticleForm` has a repeatable link editor (label with datalist suggestions, URL,
  visibility) in all three form branches; it always submits the full `links` array
  and shows the per-product limit hint.
- Settings → Kårinställningar shows the failure table plus "Check now". Active
  failures put the section at the top; when everything is fine (or only
  acknowledged failures remain) it moves to the bottom. Acknowledged rows are
  muted and can be undone.

## Seeding and tests

- `dev-seed.sh` adds links to Stormkök 1 (public + trusted), Sibley 1 (public) and
  a deliberately unreachable link on Tältlampa LED so the broken-links section is
  visible in dev.
- `api/internal/handler/tests/article_links_test.go` covers visibility, the
  configured edit role, rename/merge semantics, cancellation, the create-additive
  rule, the API cap, URL collisions, availability fallback, checker totals and the
  acknowledge/undo lifecycle.
- `api/internal/linkcheck/linkcheck_test.go` covers URL/SSRF rejection.
