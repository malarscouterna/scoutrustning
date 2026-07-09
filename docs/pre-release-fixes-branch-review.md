# Review: `feat/pre-release-fixes` branch

Full review of the branch before merge to `main`. 40 commits, ~8,100 insertions
across auth/multi-tenancy, the booking auto-swap feature, and frontend routes
(`/welcome`, `/join`, `/gdpr`, `/settings`, group switching).

Reviewed in three parallel passes: auth/multi-tenancy/composite users PK,
booking-swap/notification logic, and frontend routes/components. Findings
below are ordered by severity within each area.

## Must fix before release

### 1. `GET /api/v0/users/{id}` has no access-level gate

**File:** `api/internal/handler/users.go:776` (`UserHandler.Get`)

Unlike `List` (`GET /users`), which requires manager access, `Get` has no
`auth.AccessAtLeast` / `IsManager()` check. It only gates two sub-pieces of
the response (draft bookings, the `issues` array). Everything else - name,
picture, `notification_email`, teams, and non-draft `open_bookings`
(dates/title/team) - is returned to *any* authenticated caller, including a
plain `view`-level user.

**Failure scenario:** a `view`-level team member (someone who can only
browse the catalog) enumerates member IDs and pulls other members' emails
and in-flight booking details - more than `List` intentionally exposes even
to book-level users.

No test asserts a 403 here; `users_test.go` calls this endpoint without
checking status, so the gap is untested by omission rather than by an
explicit "this should be public" decision.

**Recommendation:** gate `UserHandler.Get` the same way as `List`
(manager-only, or at minimum require `issue_resolve` / a comparable
"can see other members' details" permission). Add a test asserting
non-manager access is rejected.

**Decision:** not a blanket manager-only gate, and not even a
`notification_email` gate - within a group, seeing another member's
contact info while logged in is fine by this project's own openness norms
(same reasoning as name/picture/teams). The only piece that already had
(and keeps) a real gate is `open_bookings`: `draft`/`rejected` statuses are
only included when `claims.IsManager()` (`users.go:98-101`,
`openBookingStatusesOther` vs `openBookingStatusesManager`), and `issues`
is gated on the viewer's `issue_resolve` permission. Both of those were
already correct as implemented - **no code change needed for this item**;
closing it as reviewed-and-accepted rather than a bug.

### 2. Race condition (TOCTOU) in unit-swap selection

**Files:** `api/internal/handler/booking_swap.go:55-94`,
`api/internal/handler/bookings.go:536-559`

`FindReplacementArticle` (a plain `SELECT`) and
`SwapBookingItemArticleByArticle` (a plain `UPDATE`) run back-to-back with
no `SELECT ... FOR UPDATE`, advisory lock, or surrounding transaction. Every
swap call site goes through `h.Q` directly, not a transaction.

**Failure scenario:** the nightly `ResolveOverdueSwaps` job and a manager's
`UpdateItemReturn`/`Update` request run concurrently against the same
commercial_name/location pool. Both `SELECT` the same free unit before
either commits its `UPDATE`, so two different bookings end up assigned the
same physical `article_id` for overlapping dates - the exact double-booking
the availability checks exist to prevent. This is a pre-existing pattern
(shared with the archive-conflict flow in `articles.go`), but this branch
adds a new concurrent caller (the hourly nightly job) that meaningfully
increases exposure to it.

**Recommendation:** wrap find-and-swap in a transaction with
`SELECT ... FOR UPDATE SKIP LOCKED` on the candidate replacement row (or an
advisory lock keyed on commercial_name + location + group_id) before every
swap call, including the nightly job.

**Fixed:** `FindReplacementArticle` now takes `FOR UPDATE OF a SKIP LOCKED`.
`ResolveBlockedItemsForArticle` (used by mark-as-delayed, the condition-change
paths, and the nightly job) and the separate find+swap pair in `Update`'s
date-conflict loop each run inside their own `pool.Begin`/`WithTx` transaction,
so the lock is held from find through the swap `UPDATE` and released on
commit/rollback. `BookingHandler` and `ResolveOverdueSwaps` both gained a
`*pgxpool.Pool` parameter/field for this. Verified via the full integration
suite (all existing swap tests pass unchanged - concurrent-request coverage
wasn't added since testcontainers-go doesn't make interleaving two real
transactions deterministic to assert on).

### 3. Partial swap survives a 409 in `Update`'s conflict path

**File:** `api/internal/handler/bookings.go:482-588`

The date-change conflict loop is not wrapped in a transaction. If a booking
has items A and B both needing new dates, and A has a free equivalent but B
does not, the loop swaps A's article and *then* fails on B, returning `409
article_not_available_for_dates`. A's already-applied swap (and its logged
event) is never rolled back.

**Failure scenario:** client receives a 409 implying nothing changed, but
the booking's items have actually been mutated. Silent inconsistent state
on a request the caller believes failed cleanly.

**Recommendation:** wrap the whole conflict-resolution loop in a DB
transaction so a later failure rolls back earlier swaps within the same
request.

**Decision: no fix needed.** The 409's job is to make sure the user knows
about the remaining conflict so they can act on it (remove the item) - it's
not a promise that nothing changed. A's swap having already happened is a
fine outcome, not a bug: same equivalent-unit guarantee either way, and a
retry after removing B would just re-resolve cleanly. No transaction
wrapping added for this item.

## Should fix

### 4. Grace-period cutoff loses time-of-day precision

**File:** `api/internal/handler/booking_swap.go:124-125`

`graceCutoff := time.Now().Add(-overdueSwapGracePeriod)` carries
hour/minute precision, but it's bound as `pgtype.Date` and compared against
the date-only `b.end_date` column. The intended 48h grace period actually
enforces anywhere from ~24h to ~72h depending on time of day the job runs.
Not a crash or corruption risk, but it doesn't match the documented
behavior in `docs/delayed-return-swap.md`, and
`TestResolveOverdueSwaps_GracePeriod` only covers the clearly-within-grace
case, so the boundary isn't tested.

**Recommendation:** either document the effective day-granularity behavior
explicitly (simplest, since `end_date` is date-only anyway) or truncate
`graceCutoff` explicitly to a date boundary in code so the intent is clear
and testable at the boundary.

**Decision: document, no code change.** Day-granularity is good enough for
this feature - documented in a code comment on `overdueSwapGracePeriod` and
in `docs/delayed-return-swap.md`'s grace-period section.

### 5. Duplicated, non-httpOnly cookie writes for group switching

**Files:** `web/src/routes/+page.svelte:40`,
`web/src/lib/components/UserInfoCard.svelte:25`,
`web/src/routes/settings/+page.svelte:790`

All three independently do
`document.cookie = "active-group-id=..."` then `location.reload()`. This
duplicates logic across three files and violates this project's own
identity-handling rule ("no page or component should ever reference
personas, cookies, or auth headers directly") - the same rule the
`dev-persona` cookie already follows via a server-side pattern.

**Recommendation:** extract to a single helper (e.g. `$lib/activeGroup.ts`)
or, better, a SvelteKit form action so the cookie can be set `httpOnly` +
`secure` server-side, consistent with how the persona cookie is handled.

**Fixed:** added a `POST /group/switch` handler in `hooks.server.ts`
(mirroring the existing `POST /dev/persona` pattern) that sets
`active-group-id` as `HttpOnly` + `SameSite=Lax` + `Secure` (outside dev) via
a raw `Set-Cookie` header. `$lib/activeGroup.ts` exports a single
`switchGroup(groupId)` that POSTs to it and reloads; all three call sites
(`+page.svelte`, `UserInfoCard.svelte`, `settings/+page.svelte`) now import
it instead of writing `document.cookie` directly. `svelte-check`: 0/0.

### 6. Locale-broken success/error color detection in settings

**File:** `web/src/routes/settings/+page.svelte:1884`

```js
class="text-sm {archiveMessage.startsWith('Fel') ? 'text-red-600' : 'text-green-600'}"
```

`m.page_profile_error_prefix()` is `"Fel: "` in Swedish but `"Error: "` in
English. In an English session, a real error from `saveArchiveSettings()`
fails the `startsWith('Fel')` check and renders green as if it succeeded.
This copies an existing identical bug at line 1859 (`permMessage`) rather
than fixing the pattern.

**Recommendation:** replace the string-prefix check with a boolean state
flag (`archiveError: boolean`) set explicitly at the call site, and fix
both occurrences while touching this file.

**Fixed:** added `permError`/`archiveError` boolean state, set `false` at
the start of each save attempt and `true` in the `catch` block; the two
`startsWith('Fel')` checks now read the boolean instead of parsing the
message. `svelte-check`: 0/0.

## Worth noting, not urgent

### 7. `MarkdownArticle.svelte` renders unsanitized HTML

**File:** `web/src/lib/components/MarkdownArticle.svelte:668`
(`{@html html}`), consumed by `/guide` and the new `/gdpr` route.

`marked(md)` output is rendered directly with no `DOMPurify` (or
equivalent) sanitization pass. Low actual risk today since both source
files (`guide.md`, `gdpr.md`) are developer-controlled, not user input -
but the component is now shared infrastructure with no documented trust
boundary. If either file ever becomes editable via CMS/API, this becomes
an XSS vector.

**Recommendation:** either add a `DOMPurify.sanitize()` pass now (cheap,
future-proof) or add a one-line comment on the component documenting that
inputs must remain developer-controlled markdown.

**Decision: fix now.** User-editable markdown through this component is
expected eventually (not hypothetical), so sanitize now rather than risk
missing it later when a new caller quietly becomes user-controlled. Add a
`DOMPurify.sanitize()` pass in `MarkdownArticle.svelte` itself (single choke
point at the `{@html}` call), not at each caller, so every future consumer
is covered automatically regardless of whether its source is a file or DB.

**Fixed:** added `isomorphic-dompurify` (works both server-side during SSR
and client-side, unlike plain `dompurify` which needs a DOM) as a
dependency. `MarkdownArticle.svelte` now derives `safeHtml =
DOMPurify.sanitize(html)` and renders that instead of the raw prop - the
three current callers (`guide`, `gdpr`, `docs/gchat`) and any future one are
covered without each needing to sanitize its own markdown output.
`svelte-check`: 0/0. `pnpm run build` succeeds (confirms the package works
in SSR, not just the browser).

## Checked and found clean

- **`group_id` scoping**: correct throughout all new queries (users,
  issues, booking-swap, notifications).
- **Identity derivation**: `pickActiveGroup` only selects among groups
  present in the JWT's own claims; `X-Active-Group-Id` is stripped from
  incoming browser requests and re-set server-side from a cookie in
  `hooks.server.ts`, matching the existing `X-Dev-Role-Override` pattern.
- **`/join` flow**: cross-checks the submitted org against the JWT's own
  `claims.Orgs` before any group data is exposed or a join is accepted.
- **Account removal**: intentionally unscoped by `group_id` (by design -
  it scrubs one member's row across all their groups by JWT-derived
  member ID), well covered by `account_removal_test.go`.
- **`00016_users_composite_pk.sql`**: safe on existing data (`id` was
  already globally unique, so it trivially satisfies the new composite
  PK); FKs correctly widened to composite `(col, group_id)`.
- **No-equivalent-unit path** in the delayed/overdue/condition-change
  swap triggers fails gracefully (`swapped=false, err=nil` + blocked-item
  notification) - issue 3 above is specific to `Update`'s separate
  conflict-path loop, not this path.
- **Notification dedup** correctly keyed on `booking_item.id`, not
  booking ID.
- **Svelte 5 reactivity**: `$derived` used correctly throughout new
  components; the `state_referenced_locally` cases in
  `CopyBookingModal.svelte` are intentional one-shot snapshots, explicitly
  marked with `svelte-ignore`.
- **i18n coverage**: no hardcoded strings found in new components/routes;
  the branch actually fixes two pre-existing hardcoded strings elsewhere.
- **Accessibility**: no missing label/id pairs in new forms.
- **`jwt.ts`**: only used server-side to prefill display fields from an
  already-verified Auth.js session; never a security decision point.

## Suggested order of work

1. Fix #1 (PII access-level gate) - straightforward, high impact, small
   diff.
2. Fix #2 and #3 together (transaction/locking around swap logic) - same
   root cause (missing transactional boundary around swap operations),
   worth doing in one pass.
3. Fix #5 and #6 (frontend cleanup) - quick, improves consistency with
   project conventions.
4. Decide on #4 (grace-period precision) - likely just needs a doc update
   plus an explicit truncation in code, low risk either way.
5. #7 is optional hardening, not blocking.
