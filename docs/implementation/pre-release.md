# Pre-release checklist

Work to complete before moving from `/api/v0/` (pre-release) to v1.0.

## Working process for this checklist

When picking up an item from this doc:

1. **Ask first.** Read the item's description, then ask clarifying questions about scope, edge cases, and anything ambiguous before writing any code - don't assume. If the item's "Done in ..." notes reveal it's already partially addressed or the description is stale, say so before proceeding.
2. **Implement.** Once scope is confirmed, build it, update tests, and update this doc's entry with what was actually done (root cause, design decisions, anything that diverged from the original description).
3. **Tell the user what to check.** After implementing, give concrete steps to verify the change in the running app (which page, which persona/role, what action to take, what result to expect) - not just "tests pass". Flag any manual step needed (rebuild, migration, reseed) explicitly.

---

## Notifications

- [x] Schema cleanup (`00009_gchat.sql`) - drop stale webhook/channel columns
- [x] Phase 3.5a - three-tier prefs, team broadcast email, email threading
- [x] Phase 3.5b - Google Chat bot, team-space mapper UI
- [x] Phase 3.6 - Gruppkanal, personal email policy
- [x] Phase 3.7 - GChat two-message threading, issue broadcast parity
- [x] End-to-end smoke test in dev with real GChat space (3.7-5)
- [x] Integration tests for GChat key management endpoints (3.5b-6)

---

## Multi-group support (landing page, signup, GDPR)

Full plan, rationale, and implementation notes in `docs/implementation/scout-group-signup.md` - this entry only tracks status against this checklist.

- [x] `/welcome` public landing page, absorbing `/login` (deleted), linking to signup and GDPR
- [x] `/gdpr` info page - deployment-provided markdown (`gdpr.md`, gitignored), not hardcoded, with a template committed for reference
- [x] `/join` group signup form + admin/applicant email notifications
- [x] Group switching (multi-group users, active-group cookie, `/api/v0/me` returning all groups)
- [x] Account removal (self-service, in-place scrub rather than orphan - see below)
- [ ] Deferred UX polish (explicitly not done yet, tracked in `docs/implementation/scout-group-signup.md`): logged-out language switcher on the three public pages

All five phases of `docs/implementation/scout-group-signup.md` are now complete.

Done in a series of commits building out `docs/implementation/scout-group-signup.md` §1/§2/§4/§5:
- **Group switching (§3/§4):** `users` PK changed to composite `(id, group_id)` so one member can hold a profile row per registered group. Active group is resolved via an `active-group-id` cookie (mirroring the existing `dev-persona` pattern) forwarded as `X-Active-Group-Id`, not a DB column as originally planned - found during implementation that `auth.Middleware` already re-resolves the group fresh from the JWT on every request by iterating a Go map (randomized order), which was a real latent bug for any multi-group member; the fix (`pickActiveGroup`: hint → primary → stable first match) closes that too. Switcher UI: a compact "group name + ▾" dropdown, appearing in the `UserInfoCard` popup, `/profile`, and the dashboard root.
- **Account removal (§5):** turned out much simpler than planned. Rather than delete the `users` row and loosen 9 FKs to nullable (or point them at a placeholder row), `DELETE /api/v0/me` scrubs personal fields (name/email → fixed placeholder, resolved via the group's language; picture/notification settings cleared; team access reset to `view`) on the same row(s), keeping every FK (bookings, booking_events, issue_reports, ...) linked with zero migration. Global across every group the member belongs to, not just the active one - a single `WHERE id = @id` update, no per-group loop - so there's no gap where removing from one group leaves you locked out of another until you "recreate" via a fresh login. Re-login naturally restores the profile via the existing upsert-on-login overwrite. UI: two-step-confirm "danger zone" on `/profile`.
- `/join` required a real architecture decision, not just a form: the audience (unmapped-group users) is exactly who the standard `auth.Middleware` rejects with `403 group_not_found` before any handler runs. Solved with an additive `AllowUnmapped` flag on `MiddlewareConfig` plus a `Claims.Orgs` field populated from the raw JWT `memberships` claim - zero behavior change for every other route, which still uses the strict instance.
- Two operational gotchas worth remembering for future env vars: `docker-compose.yml`'s `api` service uses an explicit `environment:` allowlist rather than `env_file: .env`, so a new var in `.env` silently does nothing until it's also added there; and `web`'s Paraglide messages are generated from `api/internal/i18n/messages/*.json` at build time, so new i18n keys need a container rebuild/restart, not just a source edit, to show up.
- Signup is refused outright (`503`) if `ADMIN_EMAIL` isn't configured - deliberately no silent "half-succeeded, applicant got a confirmation but nobody will ever act on it" state.
- Iterated several times on form details based on manual review: role field is a required dropdown of the applicant's actual Scoutnet roles (not free text, not hardcoded to "manager" - that was a real bug, the generated `init-group --role-key` was always `manager` regardless of what the applicant selected); a separate free-text "manager team name" field (e.g. "Utrustningsgruppen") distinct from the Scoutnet role title; contact email deliberately not prefilled from the applicant's own address (nudged toward a shared group mailbox instead); custom domain collection reduced to an interest checkbox with no domain name field, explicitly not wired into `init-group` - that's a manual follow-up conversation.

---

## Repo hygiene

- [x] ~~Separate internal work-in-progress planning docs from docs actual users/deployers care about~~ - Done in `docs: separate internal planning docs from user-facing docs`. All planning/design docs (this file, `SPEC.md`, `BACKLOG.md`, `accomplished.md`, `scout-group-signup.md`, and 17 others) moved into `docs/implementation/`. `docs/` now holds only `README.md`, `docs/guide.md`, `docs/API.md`, `docs/gdpr.md`, `docs/gchat-manager-guide.md`, `docs/import-example.csv`, and `docs/seed-images/`.

---

## Dependency freshness

- [x] ~~Audit `api/go.mod` (`go list -m -u all`) and `web/package.json` (`pnpm outdated`) for outdated dependencies before v1.0~~

Done in `chore: dependency freshness audit`:
- **Go**: patch/minor bumps via `go get -u=patch ./... && go mod tidy` (chi v5.3.1, pgx v5.10.0, goose v3.27.2, klauspost/compress, moby/moby, gopsutil, x/sync, etc.). No major bumps pending in direct deps.
- **Web dev deps**: patch/minor bumps via `pnpm update` - `@sveltejs/kit` 2.61.1→2.69.2, `svelte` 5.55.9→5.56.4, `svelte-check` 4.4.8→4.7.2, `vite` 8.0.14→8.1.4, `@sveltejs/vite-plugin-svelte` 7.1.2→7.2.0, `tailwindcss`/`@tailwindcss/vite` 4.3.0→4.3.2, `mjml` 5.2.2→5.4.0, `@sveltejs/adapter-node` 5.5.4→5.5.7, `tsx`, `@types/node` (25.9.1→25.9.5, held below the 26.x major per the same caution as other majors).
- **`@scouterna/ui-webc` 3.2.0→4.4.4 (a major bump)**: initially flagged as high-risk since npm hosts no public changelog for it. Found the real source is the sibling repo `../j26-components` (not linked from the npm page) and read its actual `CHANGELOG.md` after fast-forward-pulling it (was a month stale locally). Every entry between 3.2.0 and 4.4.4 is either a patch fix to existing components (drawer focus-trap bugs, select/input height matching, bottom-bar sizing) or a net-new, opt-in component (Drawer, Avatar, Segmented Control, Skeleton, Pagination) - nothing renamed/removed on the existing API surface we use. Bumped along with `@scouterna/design-tokens` 0.0.6→0.0.7 (one entry: "softer gray-50", a token value tweak) and `@scouterna/tailwind-theme` 0.0.5→0.0.6 (tracks design-tokens only). No code changes needed - we don't consume any of the new components yet.
- **TypeScript pin corrected, not enforced:** `web/package.json` already had `typescript: ^6.0.3` installed, contradicting the old "`must be ^5.x`" note in `CLAUDE.md`/`.ai/project-context.md`. Verified 6.x builds and type-checks cleanly (`pnpm run build`, `pnpm run check` - 0 errors/0 warnings), so the doc was stale, not the dependency wrong. Updated both docs to state the real constraint: SvelteKit needs `^5.3.3` as a *floor*, and 6.x already works - didn't bump further to the newly-available 7.0.2 in this pass.
- **`@inlang/paraglide-sveltekit` is deprecated upstream** (confirmed via npm + inlang's own migration guide) - the recommended replacement is `@inlang/paraglide-js` v2+ used directly via its own Vite plugin, dropping the SvelteKit-specific adapter entirely. This is a real migration, not a version bump: config field renames (`sourceLanguageTag`→`baseLocale`, `{languageTag}`→`{locale}` path pattern), removal of the `<ParaglideJS>` wrapper and `i18n.ts` helper in favor of `paraglideMiddleware` in `hooks.server.ts`, and `languageTag()`→`getLocale()` plus manual `localizeHref()`/`localizeUrl()` calls wherever links are built (previously automatic). Generated `$lib/paraglide/messages.js` and `import * as m from ...` call sites should mostly survive unchanged. Estimated medium effort/risk (hours, not days) - deliberately **not done in this pass**; tracked as its own backlog item below since it touches routing/hooks, not just a dependency version.
- Bumping `@sveltejs/vite-plugin-svelte` to 7.2.0 surfaces a build-time warning that it removed the `plugin.api.sveltePreprocess` API the deprecated paraglide adapter still uses - confirmed cosmetic (`pnpm run build` still produces a correct `messages.js`, `svelte-check` 0/0), but it's another point in favor of doing the paraglide migration sooner rather than later, since a future vite-plugin-svelte release could turn this into a real break.
- Verified: Go integration suite, `smoke-test.sh` against the running dev stack, `pnpm run build`, and `svelte-check` (0 errors/0 warnings) all pass after the bumps.
- Deferred, not part of this pass (all majors needing dedicated attention, added to `BACKLOG.md`): `@inlang/paraglide-sveltekit` → `@inlang/paraglide-js` v2 migration, `typescript` 6.x → 7.x, `@types/node` 25.x → 26.x, `marked` 17.x → 18.x.

---

## User guide updates

- [ ] `docs/guide.md` needs to catch up with everything shipped in this round of work before release: booking comment thread + approval flow redesign, auto-archive countdowns, copy booking, personal bookings, collaborative bookings (once done), the user info card, delayed-return auto-swap behavior (what a user sees when their item gets silently swapped), and CSV import column reference (already tracked separately above). Audit section-by-section against the "Implementation order" list below rather than guessing what's missing.

---

## Contact / admin-email scoping

Two distinct contact concepts exist and are currently conflated under a single `ADMIN_EMAIL` env var:

- **`notification_email_from`** (existing group setting) - the address automated notifications are sent *from*. Not expected to be read/replied to by anyone - stays as-is.
- **`ADMIN_EMAIL`** (existing env var) - is actually the **system/platform admin contact**, not a group-level setting. Scope clarified:
  - Shown on the public `/join` page, so a prospective applicant (not yet a member of any group) knows who to contact about the system itself before/while applying.
  - Also surfaced to a group's own manager-access team(s) (e.g. in `/settings`) as their escalation contact for system-level problems - not shown to regular members (leaders/bookers/trusted).
- **Missing:** regular end users have no in-app way to contact *their own group's* equipment managers (a group-level concern, distinct from the system admin). Add a "contact your equipment managers" surface (e.g. on `/guide` or a footer/help link) listing the individual members of the team(s) with `manager` access level, by name and personal notification email - **not** the Gruppkanal/GChat channel, which is a fine broadcast destination for the system but not where a user with a question would expect a reply. A user with a question wants to pick a specific person to email directly. What matters here is the **manager access level itself**, not any individual manager.
- At group creation (`/join` flow), pre-fill the first manager team member's notification email from the signup form's contact email, so the contact surface isn't empty from day one. No server-side guard against later clearing it - if a group ends up with no manager email set, that's assumed to be a deliberate choice by the group, not something worth enforcing against.

Not yet scoped as an implementation task - needs a short design pass on exact UI placement of the contact list before picking up.

---

## Other frontend gaps

- [x] ~~Web header logo~~ - Done. Rendered on the dashboard root (`/`), inline with the primary CTA row, to the left of the group name/switcher (not the top nav, per the placement decision made when the CTA row was restructured). No backend/settings-fetch needed for this page - `web/src/routes/+page.svelte` hits the existing unauthenticated `/api/v0/public/groups/{groupId}/logo` endpoint directly using `data.user.group_id` (already available from the layout), and hides itself via `onerror` when no logo is set (404) rather than requiring a separate "does a logo exist" check. `alt` text is the group name (`data.user.group_name`) for accessibility. A `$effect` resets the failed-state flag when `group_id` changes, so switching groups retries loading the new group's logo instead of staying hidden from a stale failure. `svelte-check`: 0 errors/0 warnings.
- [x] ~~Logo upload UI~~ - Done. New "Logo" section at the top of the `/settings` "group" tab (before Teams): preview (or a placeholder box when unset), file input (any image type, no client-side crop - the server-side `ProcessLogoImage` pipeline handles resizing), and a remove button with confirm. Added `uploadGroupLogo`/`deleteGroupLogo` to `$lib/api/client.ts` (multipart POST / DELETE against `/group-settings/logo`, reusing the existing image-upload fetch pattern used for issue/product images) and `logo_url` to the `GroupSettings` client type (was returned by the API already but missing from the TS interface). **Resolution check:** confirmed no quality loss - `ProcessLogoImage` (`api/internal/images/process.go:243`) produces a lossless WebP capped at 1600×300 for the web variant and a PNG capped at 600×120 for email, both downscale-only (never upscales), shared with the rest of the image pipeline (govips, MIME sniffing, 25MB cap) rather than a bespoke path. Added help text recommending a wide/landscape logo (not square or tall) since the bounding box is 1600×300 - a square upload gets heavily letterboxed. `go build`, `svelte-check` (0/0), and the full Go integration suite all pass.
- [x] ~~Rename `/profile` route to `/settings`~~ - Done. `web/src/routes/profile/` moved to `web/src/routes/settings/` (same page, same three tabs, no restructure). Updated the two nav link references (`+page.svelte` dashboard button, `UserInfoCard.svelte` settings link), the three `smoke-test.sh` entries, and the outbound email unsubscribe/CTA links in `api/internal/notifications/template.go` (6 occurrences of `BaseURL+"/profile"` → `/settings`). `docs/implementation/SPEC.md` given `**UPDATE**` notes at both places it named `/profile`; other docs (`docs/implementation/notifications.md`, `docs/implementation/i18n.md`, `docs/implementation/inventory-management.md`, `docs/implementation/access-levels.md`, etc.) still reference `/profile` in historical/narrative text - left as-is since they're planning records, not living references. `svelte-check`: 0 errors/0 warnings.
- [ ] CSV import instructions - real user testing showed the import flow (profile/settings "group" tab) is hard to understand as-is: just a file picker and result feedback, no inline explanation of expected columns, duplicate-detection behavior, or the two-phase preview/confirm flow. The actual format documentation only exists in `docs/implementation/inventory-management.md`, an internal planning doc users never see. Fix: brief inline help text/panel directly in the import section, plus a link to a fuller explanation added to `docs/guide.md` (which currently only has a one-line feature bullet, no column reference).
- [x] ~~Own-profile avatar in top nav~~ - **superseded, see `docs/implementation/scout-group-signup.md` §3**: show the logged-in user's `UserAvatar` in the top nav's right-hand cluster (after `DevPersonaSwitcher`, so it doesn't collide with it in dev mode), clicking opens `UserInfoCard` for their own user ID (not a navigation to `/profile`) - this is also where the group switcher lands once multi-group support exists.

---

## Booking flow correctness

### Self-conflict on edit

When changing dates, title, or unit on an existing booking, the availability check incorrectly flags the booking's own items as conflicts because the `ExcludingBooking` path is not applied consistently during updates. Needs a fix and tests covering: date change with no conflict, date change with a real conflict from another booking, and title/unit change that should never trigger availability errors.

Done in `fix(api): apply ExcludingBooking consistently on booking date change`:
- Root cause was not a missing call site but the query itself: `AvailableArticlesExcludingBooking` had a second exclusion clause that filtered out articles already in the booking being edited - correct for `AddItems`/`SwapItem` (offering *new* items), wrong when `Update` reused it to revalidate the booking's *existing* items against new dates.
- Fixed by adding an `exclude_own_items` boolean to the single query rather than duplicating it: `false` for `Update`'s own-item revalidation, `true` everywhere items are offered to add (`AddItems`, `SwapItem`, the `articles.go` add-item picker endpoint - the last one was missed on the first pass and caught by the full test suite).
- Bookings have no `title` field, so that part of the description didn't apply; covered date-change-no-conflict, date-change-real-conflict, and unit-change-no-availability-check instead.

**Regression observed (2026-07-06):** the "Inte tillgänglig för de valda datumen" conflict error reappeared when saving a draft's notes with dates unchanged. Not yet root-caused - reporter noted it may not be specific to a notes-only save, so don't assume the cause is scoped to that one field. One lead worth checking: `Update`'s `datesChanged` check (`api/internal/handler/bookings.go`) compares `pgtype.Date` structs with `!=`, which embeds a `time.Time` - built-in `==`/`!=` on `time.Time` can spuriously report a change even for an identical instant depending on how the two values were constructed (should use `.Time.Equal()` instead, or compare via formatted date strings). Needs proper reproduction and a fix, deferred to the same future commit as the `notes` → `title` rename below since both touch the same code path.

**Second regression found (2026-07-07), after the `datesChanged` fix landed:** a title-only save still showed the conflict warning in the UI, even though the server-side update succeeded (confirmed by reload showing the new title with no conflict). Root cause was a *second*, independent bug on the client side: `book/+page.svelte`'s `saveDetails()` re-checks the cart against `GET /articles/availability/articles?exclude_booking_id=...` after every save to populate the conflict-warning banner - but that endpoint hardcoded `exclude_own_items: true` server-side (`api/internal/handler/articles.go` `AvailableArticlesList`), which is correct for its other callers (`AvailabilityPicker`, `PickupChecklist` swap) that are offering *new* items, but wrong for `saveDetails`'s self-revalidation use: with the booking's own items excluded from the "available" set, every item it already holds is deterministically flagged as unavailable regardless of any real conflict. Fixed by adding an `exclude_own_items` query param (default `true`, preserving existing behavior for all other callers) and passing `exclude_own_items: false` from `saveDetails()`. Covered by a new test in `pickup_test.go` (`exclude_own_items=false includes the booking's own items in the result`).

**Third gap found (2026-07-07):** a real date conflict (e.g. changing dates to a range where "Lykta" is genuinely unavailable) surfaced only as a generic translated error string ("Lykta is not available for the new dates") with no indication of which cart row it referred to, and no guidance on what to do next. The per-row orange highlight + "date conflict" label already existed (`BookingItemsList.svelte`, driven by `conflictingIds`) but was never populated on this path, since the hard 409 from `Update` short-circuits before the informational `listAvailableArticles` re-check that normally populates it. Fixed by parsing the `name` param off the `article_not_available_for_dates` error (already present in the response body) and matching it against `cartItems` by `common_name` to populate `conflictingIds` - this reuses the existing hint banner text ("Remove the marked articles or change the dates back...") and row highlight without adding new UI. Applied in both `saveDetails()` and `submitBooking()` via a shared `flagConflictFromError()` helper.

**Fourth gap found and fixed (2026-07-07):** the availability loop in `Update` returned on the *first* unavailable item found, so a booking with two or more simultaneously-conflicting items only ever reported one per save attempt - the user had to fix-and-resave repeatedly to discover each subsequent conflict. Changed the loop to collect all conflicting items before returning. The error response now always includes `name` (first item, for backward-compatible singular messages) plus `names`, `count`, and a comma-joined `article_ids` list; the error key switches to the new plural `articles_not_available_for_dates` i18n key when more than one item conflicts. `flagConflictFromError()` in `book/+page.svelte` now reads `article_ids` directly instead of matching a single `name` against `cartItems`, so every conflicting row is highlighted in one round trip. Covered by extending the existing "change dates fails when items not available" test in `bookings_test.go` (that scenario already had all 3 booked items conflict simultaneously, so it now also asserts `count`, the plural error key, and 3 `article_ids`).

### Booking title field

Bookings currently have a `notes` field, not a `title`. The `book`/booking-detail UI functionally uses it as a title (single-line input, shown prominently), so rename `notes` → `title` end-to-end (migration, sqlc queries, handler request fields, frontend labels/messages, `UserOpenBooking`/`BookingCard` usages) and make it required (non-empty) on submission - currently it's optional and has no validation. Bundle the self-conflict regression fix above into this same commit since both touch `Update`'s request handling for this field.

Done in `fix(api,web): rename booking notes to title, require non-empty, fix self-conflict regression on update`:
- **Self-conflict regression root-caused:** `Update`'s `datesChanged` check compared `pgtype.Date` structs with `!=` (`api/internal/handler/bookings.go:433`). The frontend (`web/src/routes/book/+page.svelte` `saveDetails`/`submitBooking`) always resends `start_date`/`end_date` on every save, even a title-only edit - so on every save the request's dates are freshly parsed via `time.Parse` while `booking.StartDate`/`EndDate` come from a pgx DB scan; comparing the two `time.Time` values with `!=` compares internal representation (not just the instant), which can spuriously differ even when the calendar date is identical. Fixed by comparing via `.Time.Equal()` instead of struct `!=`.
- Column rename via `ALTER TABLE bookings RENAME COLUMN notes TO title` (migration `00015_booking_title.sql`). Required-non-empty is enforced at the API layer (`Create` rejects empty/whitespace-only title; `Update` rejects an explicitly-empty title but omitting the field entirely is still a valid partial patch) - not a DB `CHECK` constraint, to avoid touching existing rows.
- Renamed end-to-end: sqlc queries (`bookings.sql`, `users.sql`), Go handler (`Create`/`Update`/`Copy`), frontend types/client (`client.ts`), `book/+page.svelte` state and labels, `bookings/[id]/+page.svelte`, `bookings/+page.svelte`, dashboard (`+page.svelte`), `BookingCard.svelte`, `UserInfoCard.svelte`, notification email templates (`template.go`, `booking.html`/`booking.mjml` - `EMAIL_NOTES_BLOCK` → `EMAIL_TITLE_BLOCK`), i18n keys (`page_book_notes*` → `page_book_title*`, `email_notes_heading` → `email_booking_title_heading`), and all integration tests.
- **Also removed while in this area:** `booking_items.notes` was a dead column - schema existed but no query ever read or wrote it, and `UpdateItemReturn`'s `notes` request field was decoded but never persisted. Dropped the column and removed the dead field from the Go request struct, the `updateItemReturn` API client type, and the `ReturnChecklist.svelte` form state (was never actually bound to an input, so nothing user-visible changes).
- **Also fixed in the same commit** (found during manual verification, see "Second regression" and "Third gap" above): the post-save conflict check's `exclude_own_items` flag (new query param on `GET /articles/availability/articles`, default `true`), and row-level highlighting for hard date-conflict rejections via a shared `flagConflictFromError()` helper in `book/+page.svelte`. Known remaining limitation (not fixed, tracked in `docs/implementation/BACKLOG.md`): the server only reports the first unavailable item per save attempt, not all of them at once.
- **Verified:** full Go integration suite (`go test ./internal/handler/tests/`) and `bash smoke-test.sh` both pass against a rebuilt `docker compose` stack (migration confirmed applied, `bookings.title` column present, `booking_items.notes` dropped). `svelte-check` clean (0 errors, 0 warnings).

### Copy booking flow

The copy API endpoint (`POST /api/v0/bookings/{id}/copy`) exists but the UI is not exposed. A user should be able to copy any existing booking to create a new one with the same items, then set new dates.

When the UI is built, the user must be able to set the new date range before conflict-checking fires - otherwise the copied items are immediately flagged as conflicting with the source booking. Items that are unavailable for the new dates should be marked, not silently included.

Tests needed: copy to clear dates, copy to dates that overlap the source, copy where some items are unavailable.

Done in `feat(web): copy booking UI`:
- **Design decision on "clear dates":** making `bookings.start_date`/`end_date` truly nullable would have required dropping the `NOT NULL` + `CHECK (end_date >= start_date)` constraints from migration `00001_init.sql` and auditing every availability/archive-countdown query that assumes non-null dates - a much bigger, riskier change than the copy feature itself. Instead, a `CopyBookingModal.svelte` popup forces the user to re-enter title, unit, and a real date range *before* the copy is ever shown in the cart builder: it calls `POST /copy` (still creating the draft with the existing today+7 placeholder dates server-side, unchanged), then immediately calls `PATCH /bookings/{id}` with the user's chosen title/unit/dates before the modal closes. No schema change, no nullable dates.
- This ordering satisfies "set dates before conflict-checking fires" for free: conflict-checking has only ever run on an explicit save (`saveDetails` in `/book`), never automatically on load, so a freshly-copied draft sitting with placeholder dates was never actually a problem in practice - but the modal now means the placeholder dates are never shown to the user at all, since they're overwritten before the browser ever navigates to the new booking.
- "Items unavailable for the new dates should be marked, not silently included" is handled by the existing `Update` conflict-checking (item 3/8's `exclude_own_items` work): if the modal's chosen dates conflict with another booking holding the same items, the `PATCH` call 409s with `articles_not_available_for_dates` + `article_ids`, surfaced as an inline error in the modal so the user can pick different dates before the draft is ever navigated to. The draft itself still exists (not rolled back) so retrying just re-submits the `PATCH`, not a fresh copy.
- Copy action exposed as a "Kopiera bokning" text link in the booking detail page's action row (any status, gated on `canBook(user)`), and as a small copy icon on `BookingCard` (dashboard pending/active sections, and the full `/bookings` list). Deduplicated the `/bookings` list page's previously-inline card markup into `BookingCard` in the same commit (was an exact duplicate, now takes the new `onCopy` prop).
- On success, navigates to `/bookings/{new-id}` (the detail page), not straight into `/book` - the user can review the copy and click "Redigera" from there like any other draft.
- New Material Symbols icon `content_copy` added to the self-hosted `web/static/material-symbols-outlined.woff2` subset (previously only contained `camping`).
- Backend tests added to `TestBookingFlow_Copy`: copy + set non-overlapping dates succeeds; copy + set dates overlapping another booking holding the same items 409s with populated `article_ids`. The "copy to clear dates" scenario from the original ask is superseded by the popup design above (dates are never literally cleared/null; they're always re-entered as a real range).
- Full Go integration suite and `smoke-test.sh` both pass.

**Resolved by item 11:** `Copy` copies each item by its exact `article_id` (e.g. specifically "Sibley 1"), not by `commercial_name` + `location` the way a normal add-item pick does. So if that exact physical unit was unavailable for the dates chosen in the modal, the `PATCH` used to 409 on that specific item even when another unit of the same product (e.g. "Sibley 2") was free. Item 11's `Update` conflict-path swap (decision 7, done) now swaps to an equivalent unit before falling back to the 409, universally - not a copy-specific fix, so no changes were needed here. See `docs/implementation/delayed-return-swap.md` decision 7.

### Booking status - cancel button state machine

`cancellable` currently excludes only `returned` and `cancelled`. Correct behaviour:

- `draft`, `submitted`, `approved`, `confirmed`, `rejected` - cancellable. Rejected bookings can be walked back (the manager may change their mind, or the user wants to clean up).
- `picked_up` - not cancellable while items are out. The user must complete the return flow first.
- `returned`, `cancelled` - already terminal, no cancel button.

Separately, user feedback reported "avbokningsknapp saknas" - a cancel button missing somewhere it should exist. The specific context is unknown. Needs reproduction to ensure both the spurious and missing cases are resolved together.

Done in `fix(web,api): cancel button - correct cancellable status allowlist`:
- `cancellable` changed from "not returned/cancelled" to the explicit allowlist above; server-side `Cancel` now also rejects `picked_up` (previously UI-only).
- The likely cause of "avbokningsknapp saknas": `web/src/routes/book/+page.svelte` (the cart-builder page, reachable via `/book?id=` for any editable booking, not just drafts) had its own Cancel button with **no status check at all**, disagreeing with the booking detail page about when Cancel should show. Now uses the same allowlist.

### Booking comment thread and approval flow redesign

The current booking page conflates three distinct concerns into one undifferentiated area. The redesigned structure has three clearly separated sections:

**1. Booking details** - title, dates, unit, items. Editable when the booking is in `draft` or `rejected` state.

**2. Comment thread** - always visible, even before the booking is submitted. Users can add context while building the booking in draft. Comments are chronological. Approval events (submit, approve, reject, resubmit) appear inline in the thread as structured entries with a distinct visual style (not plain text bubbles). This makes the full history readable: a rejection followed by a resubmit is visible in order without context loss.

**Essential:** the user must be able to post a comment while the booking is still in `draft`, independent of submitting - i.e. a free-standing "add comment" action in the thread itself, not only the optional message field bundled into the submit action in section 3 below. Without this, there's no way to leave context before the booking is ready to submit (e.g. "waiting on confirmation from X before I finalize dates").

**Also essential:** adding or removing an item from the booking must produce a thread entry (structured event, same visual style as approval events), so it's visible in the history that a user changed the item list - not just that the booking exists in its current state. The `items_changed` event type already exists in the `booking_events` check constraint but is unused; wire it up on `AddItems`/`RemoveItem` (`SwapItem` is the delayed-pickup swap flow, item 10's concern, not item-list editing). Deferred to backlog: making these entries clickable to show an actual diff (which article was added/removed) - for now, a plain count summary is enough.

Consecutive add/remove actions by the same actor within a short window (10 min) collapse into a single thread entry rather than one row per click - otherwise the thread floods during active cart-building. A different actor, a different event type in between, or exceeding the window each start a fresh entry, so concurrent editors don't clobber each other's entries. Wording differs by phase: before the booking has ever been submitted, the message is a static "Påbörjade bokning" (no changing item count - a running "Skapade bokning med N" would visibly flicker as the count changes while someone is still actively building). Once it's been submitted at least once, later item changes use add/remove delta wording ("La till X föremål, Tog bort Y föremål"). The item count *at the moment of submission* is captured by extending the `submitted` event's own message instead (e.g. "Behöver detta för hajk - 3 föremål"), not by a separate items_changed entry.

**3. Approval action area** - shown below the thread, contextual per `(status x role)`:
- `draft` (user): submit button + optional message field. The "Vill ha bekräftelse från ansvarig" checkbox is shown but auto-checked and non-interactive (with hover tooltip explaining why) when any item in the booking requires approval. When no item requires approval, the checkbox is optional.
- `submitted` (manager): approve/reject buttons + optional message field.
- `rejected` (user): the booking re-enters a draft-like editable state. The user can edit items and dates, add a comment, and resubmit. It should be visually clear that the items are still on hold and will be released if not resubmitted within the auto-archive window (see below).
- `approved`/`confirmed` (user): no approval actions, just the pickup button.
- Terminal states (`returned`, `cancelled`): read-only thread, no action area.

The comment thread should be shown on the booking list card as a preview (last comment, unread indicator) to make it clear that the booking is a living conversation, not just a status.

**Editing after submission:** A booking can be edited (items, dates) while in `submitted` state without needing to withdraw it. This should be visually clear - the edit controls should remain accessible in submitted state, not hidden or greyed out. The manager sees the updated booking when they review it.

**Auto-approval visibility:** When submitting, the UI should clearly indicate whether the booking will be auto-approved (trusted team + all items at `low` approval level) or requires manager review. This can be shown as a short confirmation line near the submit button, e.g. "Bokningen godkänns automatiskt" vs "Bokningen skickas för granskning". The "Vill ha bekräftelse" checkbox (when optional) lets the user override auto-approval and request a manual review anyway.

Done in `feat(api,web): booking comment thread and approval flow redesign`:
- `items_changed` booking events wired up on `AddItems`/`RemoveItem`, merged per actor within a 10-minute window into a single row (`BookingHandler.logItemsChangedEvent`) with a count-based message stored structurally in `metadata` (`itemsChangedCounts{Added, Removed, PreSubmission}`) rather than a list of item names - ready for the future clickable-diff view (still backlogged) without a schema change. Wording: static "Påbörjade bokning" pre-first-submission (avoids a flickering count while someone is actively building their cart), add/remove delta wording ("La till X föremål, Tog bort Y föremål") afterward. `SwapItem` untouched - it's the delayed-pickup swap flow (item 10's concern), not item-list editing.
- `Submit` now always logs its `submitted` event (previously skipped when auto-confirmed with no message) and appends the item count at that moment (e.g. "Behöver detta för hajk - 3 föremål") - this is where the "what state was it sent in with" information lives, not on a separate items_changed entry.
- `GetBooking` now returns `auto_approves`, computed the same way `Submit` decides confirm-vs-submit, so the frontend doesn't need a separate endpoint for the confirmation line.
- **Design correction made while implementing:** `RejectBooking` previously set `status = 'draft'` directly on reject, silently discarding a distinct rejected state. Changed to persist `status = 'rejected'`; it now stays `rejected` until the user actually starts editing (`Update`/`AddItems`/`RemoveItem` reopen it to `draft` via `reopenIfRejected`) - not on view, not on a straight resubmit. This also unblocks the auto-archive item below, whose "timer starts when the booking is rejected" needs a real status transition to hang off of.
- Frontend (`bookings/[id]/+page.svelte`) restructured into the three sections; comment thread (with the free-standing add-comment box, already functional server-side pre-redesign) always visible above the approval action area, not interleaved with it. `items_changed` events render without the generic action-label prefix since the message is already self-describing.
- Checkbox lock is a per-item check (`any item.approval_level !== 'none'`), independent of the `auto_approves` confirmation line (which additionally factors in team trust and the personal-booking-always-needs-approval rule) - the two intentionally diverge for a trusted team booking a `low`-level item: checkbox locked-checked (forcing manual review) while, absent the lock, that combination would otherwise auto-approve.
- **Gap caught during manual verification:** the dashboard's draft quick-link (`web/src/routes/+page.svelte`) goes straight to the `/book` cart builder, not `/bookings/{id}` - so the thread wasn't reachable from there. Rather than redirect that link, extracted the thread into a shared `BookingCommentThread.svelte` component and added it to `/book` as well, so it's visible while actively building a draft, not just from the detail page. Also added a "view full booking" cross-link from `/book` to `/bookings/{id}` (the reverse direction - detail page to cart builder - already existed via the "Redigera" button).
- List-card comment preview (last comment + unread indicator) split out to its own commit - see Implementation order.

### Booking auto-archive setting

The existing 48-hour cleanup for empty drafts (no items) stays as a hard-coded system behaviour and is unaffected by group settings.

Two separate group-level settings cover bookings with items:

- **Draft with items** - default 3 days. Timer starts when the first item is added.
- **Rejected awaiting resubmission** - default 7 days. Timer starts when the booking is rejected.

**Resolved in item 7:** `RejectBooking` now persists `status = 'rejected'` (previously went straight to `draft`, which this section's "timer starts when the booking is rejected" depends on being a real, distinct status). It stays `rejected` until the user starts editing it - `Update`/`AddItems`/`RemoveItem` transition it to `draft` at that point (`BookingHandler.reopenIfRejected`), not on view or straight resubmit. So the timer for this setting starts at the `rejected` status's `updated_at` (or the most recent `rejected` `booking_events` row, if `updated_at` proves too coarse once other fields can change post-rejection without leaving `rejected` - shouldn't happen given the reopen-on-edit logic, but worth double-checking when implementing this item).

The deadline is fixed from the moment the stage is entered and does not reset on edits. This keeps the countdown predictable and honest - the user knows exactly when their items will be released regardless of what changes they make.

The booking detail page shows a countdown to the deadline - including hours and minutes - whenever the booking is in one of these timed states. The display should feel urgent enough that the user takes it seriously.

The draft countdown in particular should combine urgency with reassurance: the message should make clear that submitting is the safe move, that it is not a final commitment, and that items remain on hold either way. The goal is to push users toward submitting rather than leaving bookings to expire in draft. Something like "Skicka in din bokning för att hålla den aktiv - du kan fortfarande ändra den efteråt."

When a booking is auto-archived its items are released and the event thread records the archival clearly (not a silent disappearance). Setting 0 for either disables auto-archiving for that stage.

Done in `feat(api,web): booking auto-archive setting`:
- `group_settings.draft_archive_days` (default 3) / `rejected_archive_days` (default 7), 0 disables. Settings UI section added to `/settings`.
- An hourly job (`handler.ArchiveExpiredBookings`, reusing the ticker previously used for the 48h empty-draft cleanup) cancels expired bookings, releasing items and logging an `auto_archived` booking event. Runs immediately on startup too (not just after the first tick), and every 1 minute instead of hourly in genuine local dev (`devMode && !demoMode`), to make the feature fast to verify.
- A one-time advance warning ~24h before the deadline (`notifications.SendArchiveWarnings`, also hourly with a narrow 23-24h detection window for precision), broadcast to the team's channels (email/GChat) plus personal email to creator+team, deduped via `notification_log`.
- **Revised during implementation - the 48h empty-draft cleanup is gone, not "unaffected."** Originally planned to keep the old hard-coded 48h empty-draft cleanup as a separate, unconfigurable behavior alongside this feature. In practice that meant a brand-new empty draft showed no countdown at all until the first item was added (the draft-archive timer started at `first_item_added_at`, not booking creation) - inconsistent with "the booking detail page shows a countdown... whenever the booking is in one of these timed states," and a worse experience once this feature existed. Simplified to one rule: the draft deadline runs from `bookings.created_at` (a `bookings.first_item_added_at` column was added then removed - migrations `00018`/`00019` - once this became clear), covering empty and non-empty drafts alike, superseding the old 48h cleanup entirely. The countdown is now visible "from the beginning" as originally intended by the "whenever" wording above.
- Booking-detail-page countdown (`bookings/[id]/+page.svelte`): live-updating (30s tick, minute-level granularity - no seconds), amber background escalating to red under 24h remaining. Also shown on `/book?id=` (the cart-builder page), not just the read-only detail page, since that's where a user is actually working on the booking.

### Personal bookings - concept completeness

A personal booking covers a member borrowing equipment for scout or external use that is not associated with any registered team - e.g. a leader running a personal activity or borrowing for an external group.

**Policy:**
- Personal bookings always require manager confirmation regardless of article `approval_level`. This must be enforced server-side, not just in the UI.
- A group-level access switch controls who can create personal bookings: managers, trusted, bookers, or viewers. Follows the same pattern as the existing per-team access level switches.

**UX:**
- "Personlig bokning" appears as the last option among the user's own units in the unit dropdown, before any other-teams section (managers only see other teams below a divider). No divider is needed between own units and the personal option itself.
- In the bookings list, personal bookings show the creator's name so managers can distinguish between users' personal bookings.

Done in `fix(web,api): personal bookings - group access switch + server-side approval enforcement`:
- `needsApprovalForLevel` now forces approval whenever a booking has no team, regardless of article `approval_level` - enforced server-side across create/update/submit/add-item/remove-item flows.
- New `personal_booking_role` group setting (`view`/`book`/`trusted`/`manager`, default `book`) gates who can create a personal booking at all; checked server-side in `Create`, exposed to all users via `/me` permissions (needed client-side to decide whether to show the dropdown option), and configurable by managers in group settings.
- Booking list queries now join `users` for `creator_name`, shown on personal bookings in both the dashboard and the full bookings list.

### Terminology - avdelning/roll

"Avdelning" is used both as the specific label for `type = 'troop'` teams and generically for any team throughout the Swedish UI. In scout terminology, "avdelning" is specifically a troop-level unit - the generic use collides with this.

**Decision: no umbrella term.** "Enhet" read poorly in practice and "grupp" collides with the existing `groups` (scoutkår) concept. Instead, spell out the distinction explicitly per context:
- Full form in prose/help text: "avdelning eller roll" / "avdelningar och roller" (conjunction depends on whether the sentence is inclusive-or or listing both).
- Hyphen-joined in compound headings: "Avdelnings- och rollnotiser".
- Slash form in tight UI (table columns, parenthetical): "Avdelning/roll".
- The manager/admin team is treated as a role specifically (not generic) - e.g. "admin-rollen" in "sista admin-rollen" messages.

Done in `fix(web): consistent avdelning/roll phrasing, sort troops before roles`:
- Audited all keys in `sv.json` that used "avdelning" generically and reworded per the patterns above.
- `type = 'troop'` continues to display as "Avdelning"; `type = 'role'` continues to display as "Roll". These type-specific labels are unchanged.
- The team settings page keeps its existing heading ("Avdelningar och roller") rather than being renamed. Division between troops and roles is achieved by sorting troops before roles within each access-level column, with a visual divider between the groups - no separate page sections needed.

### Collaborative bookings

It should be possible to add other teams (troops or roles) or specific people to a booking. Added participants can modify the booking, add/remove items, and perform pickup. This is necessary when multiple teams are collaborating on an activity.

- A "Lägg till deltagare" section on the booking detail page, showing current participants and an add field. No confirmation or notification when adding.
- Two participant roles: **editor** (can add and remove items, change dates, perform pickup) and **viewer** (read-only access to the booking). No finer split between add and remove - editors have full item edit rights.
- The primary booker's permission level governs what the booking can do (approval level, access gates). Adding an editor does not elevate their effective permissions beyond what the primary booker has.
- The booking appears in the bookings list for all participants, visually distinguished from own bookings.
- The UI should indicate that a booking has collaborators (e.g. a small participant count or avatar strip on the booking card).

---

## Delayed return - conflict handling

Full design plan, decisions, and implementation notes in `docs/implementation/delayed-return-swap.md` - this entry only tracks status against this checklist.

When an item is still `picked_up` at the start of the next booking's date range:

**Auto-swap:** If an equivalent article (same `commercial_name` + `location`) is available, automatically swap it into the waiting booking. Applies to both individually-tracked and quantity-tracked articles - the distinction does not affect swap eligibility. No confirmation required, no notification sent. A `swap` event is logged in the booking event thread so the change is visible but not disruptive.

The swap check runs in two situations: immediately when any user marks an item as delayed, and as a nightly job for items that are overdue but have not been explicitly marked delayed.

**Decided: shared swap-resolution helper, reused by item 10's copy flow.** The user shouldn't care which physical unit they're assigned, only that they have one - so "find an equivalent available unit and substitute it in, silently" is one operation with (at least) two callers:
1. This item's delayed-return swap (mark-as-delayed + nightly job).
2. `Copy`'s date-set flow (item 10 above): when the copy modal's chosen dates conflict with the exact source item, silently resolve to any other available unit of the same `commercial_name` + `location` instead of 409ing on that one unit. Only surface the hard conflict if no equivalent unit exists at all for those dates.

Likely also applicable to `Update`'s general conflict path (any date change on an existing booking, not just a copy) - same reasoning applies, decide when implementing whether to fold that in too or keep it scoped to copy for now. Implementation: a single `FindEquivalentAvailableArticle`-style query/helper in the availability layer, called from both sites rather than duplicated. Build this as its own commit when picking up item 11, updating item 10's copy flow to call it too.

**Notification (no swap available):** Send to the affected next booker: which items are affected and a link to their booking page. Do not include names of the current booker in the notification.

**Booking page conflict overview:** The booking detail page for the affected booking shows a warning section listing each blocked item. The warning links to a UI that shows who currently has the item - name, unit, and expected return date. Contact info shown is the user's chosen personal notification email.

**When marking an item as delayed:** The manager marking the delay is shown the next expected user of the item (if the overlap falls within the delayed window) so they can anticipate the conflict before it occurs.

---

## User info card component

Several flows need to show information about another user (current booker, person who picked up an item, collaborator on a booking). A shared component is needed rather than ad-hoc name/contact rendering.

**Full card (popup/sheet):** Name, profile picture, personal notification email, list of team affiliations with type and access level. If shown in the context of a specific booking, highlight the affiliation relevant to that booking and list other open bookings. Used on booking detail (show current holder of a delayed item), issue detail, and collaborator management.

**Compact view (selector/inline):** Name + profile picture only. Used in dropdowns, the collaborator list on a booking, and anywhere a user reference appears inline.

**Profile pictures:** Sourced from the Keycloak OIDC `picture` claim, stored alongside other user claims on login. Initials-based avatar as fallback when the claim is absent.

Done in `feat(api,web): user info card component - full card and compact view`:
- `users.picture` column added; `picture` OIDC claim parsed in `auth.go` and persisted on the login upsert alongside `name`/`email`; also exposed on `/me`.
- New `GET /api/v0/users/{id}` endpoint (any authenticated group member) returns name, picture, notification email, team affiliations (`GetUserTeamAffiliations` joining `teams` against the target user's `team_ids`), and open bookings (`GetUserOpenBookings`). A booking is included if the user owns it (`created_by`) or has participated via a non-management action logged on `article_events` (`booked`/`picked_up`/`returned` - add items, pickup, return); submit/approve/reject live on `booking_events` and are intentionally excluded. Removing an item isn't logged as an `article_event` today, so it doesn't yet count as participation - tracked in `docs/implementation/BACKLOG.md`. The open-bookings status set depends on the caller's role - managers get all non-terminal statuses including `draft`/`rejected`, others get `submitted, approved, confirmed, picked_up`.
- The endpoint also returns `issues`: open/in-progress issues the target user reported or is assigned to (`GetUserIssues`), gated on the *viewer's* `issue_resolve` permission (same `PermissionCache` check used elsewhere) - non-managers get an empty list rather than a 403, since the rest of the card is still valid for them.
- `UserAvatar.svelte` (picture or initials fallback), `UserBadge.svelte` (compact, opens the full card on click), and `UserInfoCard.svelte` (full popup/sheet, fetches on open, highlights the team affiliation matching `contextBookingId` when supplied) added to `web/src/lib/components/`. The card visually splits open bookings into a normal section and a separate "endast synligt för utrustningsansvariga" section for `draft`/`rejected` bookings, so it's clear to a manager why a non-manager viewing the same profile would see fewer rows.
- Integrated into the booking detail event thread (`bookings/[id]/+page.svelte`), replacing the previous bare `event.actor_name` text - the first real consumer, so items 10 and 11 can build on it directly. `ListBookingEvents` now also joins `actor_picture` so the inline badge (not just the popup) shows the real photo.

---

## Phase 2 remaining (inventory management)

Lower priority but useful before real users arrive.

- [ ] CSV import two-phase flow - dry-run preview with per-row duplicate detection, then confirm
- [ ] CSV export - client-side, import-compatible columns, on browse page
- [ ] Print-friendly booking fetch list - `@media print`, grouped by location, on booking detail

---


## Implementation order

Proposed commit sequence. Each item is a self-contained PR.

1. ~~`fix(web): terminology - replace generic avdelning with enhet in sv.json and UI`~~ - Done as `fix(web): consistent avdelning/roll phrasing, sort troops before roles` (no umbrella term; see Terminology section above).
2. ~~`feat(web): rename team settings page to Enheter, split by troop/roll`~~ - Covered by commit 1: troop/role division achieved via sort + divider on the existing page, no rename or restructure needed.
3. ~~`fix(api,web): booking edit - apply ExcludingBooking consistently on date/unit/title change`~~ - Done. Root cause was the query's self-item exclusion, not variant choice; consolidated into a single query with an `exclude_own_items` flag.
4. ~~`fix(web): cancel button - correct cancellable status allowlist`~~ - Done. Also fixed the `/book` cart page, which had no status check at all - likely the actual "avbokningsknapp saknas" cause.
5. ~~`feat(api,web): personal bookings - group access switch + server-side approval enforcement`~~ - Done. See Personal bookings section above.
6. ~~`feat(api,web): user info card component - full card and compact view`~~ - Done. See User info card component section above.
7. ~~`feat(api,web): booking comment thread and approval flow redesign`~~ - Done. See Booking comment thread and approval flow redesign section above.
8. ~~`fix(api,web): rename booking notes to title, require non-empty, fix self-conflict regression on update`~~ - Done. See Booking title field section above.
9. ~~`feat(api,web): booking auto-archive setting`~~ - Done. See Booking auto-archive setting section above.
10. ~~`feat(api,web): copy booking UI`~~ - Done as `feat(web): copy booking UI`. See Copy booking flow section above.
11. ~~`feat(api,web): delayed return - auto-swap, conflict overview, next-booker notification`~~ - Done. Scope grew (2026-07-08) to also cover condition-change swaps (`reported_usable`/`reported_unusable`/`missing`) and universal swap-before-409 in `Update`'s conflict path (which also fixed item 10's copy-flow gap above for free). Full design and decision log in `docs/implementation/delayed-return-swap.md`.
12. `feat(api,web): collaborative bookings - add enheter and people to a booking` - New participants model, shared pickup rights. Depends on 6 and 7.
13. ~~`feat(web): web header logo`~~ - Done. See "Web header logo" in Other frontend gaps section above.
14. ~~`feat(web): free-form image crop in issue reporting`~~ - **Stale, no change needed.** Issue reporting never had a locked-ratio crop; it already uploads free-form.
15. `fix(web): CSV import instructions` - Inline help text/panel in the import section (profile/settings "group" tab) plus a fuller column reference added to `docs/guide.md`. See "CSV import instructions" in Other frontend gaps section above.
16. `feat(web,api): CSV import two-phase flow` - Dry-run preview with per-row duplicate detection, then confirm. See Phase 2 remaining section above.
17. `feat(web): CSV export` - Client-side, import-compatible columns, on browse page. See Phase 2 remaining section above.
18. `feat(web): print-friendly booking fetch list` - `@media print`, grouped by location, on booking detail. See Phase 2 remaining section above.
19. ~~`docs: separate internal planning docs from user-facing docs`~~ - Done. See Repo hygiene section above.
20. `feat(web): logged-out language switcher on public pages` - Deferred UX polish noted in Multi-group support section above (`/welcome`, `/gdpr`, `/join`).
21. ~~`chore: dependency freshness audit`~~ - Done. See Dependency freshness section above.
22. `docs: update guide.md for booking flow redesign, auto-archive, copy, personal/collaborative bookings, user info card, delayed-return swaps` - See User guide updates section above.
23. `feat(web,api): system-admin and equipment-manager contact surfaces` - Scope `ADMIN_EMAIL` display to `/join` + manager-only settings view; add a group-manager contact list for regular members. Needs a short design pass first - see Contact / admin-email scoping section above.

Deferred out of pre-release scope, moved to `docs/implementation/BACKLOG.md` (2026-07-08): per-item descriptions for individually-tracked articles (was 14), booking list card comment preview (was 16).

---

## Release gate

- [ ] All items above complete
- [ ] Smoke tests pass on a clean `docker compose up` + seed
- [ ] API moved from `/api/v0/` to `/api/v1/`
- [ ] `init-group` run on production VPS
- [ ] Release Please PR merged → v1.0.0 tag + Docker images pushed
