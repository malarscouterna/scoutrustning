# Multi-group support: landing page, signup, switching, GDPR, account removal

Planning document. Nothing here is implemented yet — this is the plan to be approved before any code changes.

## 1. Landing page (`/welcome`)

- New public route `/welcome`, added to `isPublicPath()` in `hooks.server.ts`.
- Root `/` is unchanged: still the dashboard, still auth-gated. Logged-out visitors to `/` still get redirected same as today, but the redirect target becomes `/welcome?callbackUrl=...` instead of `/login?callbackUrl=...` (see below - `/login` is being folded into `/welcome`).
- `/welcome` absorbs essentially all of the current `/login` page content:
  - Logo, title/subtitle, "how it works" steps, dev/demo environment links, guide link, open-source link.
  - The ScoutID sign-in form (`<form action="/auth/signin/keycloak">`) with `callbackUrl` hidden field, shown whenever the visitor is logged out (i.e. always, since only logged-out users get redirected here with a `callbackUrl`).
  - When the visitor **is** logged in (they navigated here directly via a footer/nav link while authenticated), show a "Go to dashboard" link instead of the login form.
- New additions to this page's content, per the original ask:
  - Link to group signup (`/join`, see below).
  - Link to GDPR info (`/gdpr`, see below).
- `/login` route is deleted. All existing redirects to `/login?callbackUrl=...` (in `hooks.server.ts`, and the `+layout.server.ts` unmapped-group-user fallback) are repointed to `/welcome?callbackUrl=...`.
- Unmapped-group users (`oidcName` set, `user: null` case in `+layout.server.ts`) are referred to `/welcome` instead of `/`, so they land on a page that explains the service and links to group signup.
- Touch points requiring updates (hardcoded `/login` or `/` assumptions found in the codebase):
  - `hooks.server.ts`: `isPublicPath()` list, both `redirect(302, /login?...)` call sites.
  - `+layout.server.ts:92,120`: unmapped-user fallback redirects.
  - `login/+page.svelte`: content moves to `web/src/routes/welcome/+page.svelte`; `login/+page.server.ts` (if any) moves too; delete the `login/` route folder after migration.
  - `+layout.svelte`: no change needed (root nav links stay pointed at `/`).
  - `smoke-test.sh`: replace `/login` smoke check with `/welcome`.
- **Gap found during implementation (follow-up, not yet built):** logged-out visitors have no way to switch language. `paraglide_lang` is currently only ever set from a logged-in user's saved profile preference (`profile/+page.svelte`) or the browser's `Accept-Language` header - there's no control on `/welcome`, `/join`, or `/gdpr`. Fix: a small language link/toggle in the footer of these three public pages that sets the `paraglide_lang` cookie client-side and reloads, reusing the same one-liner `profile/+page.svelte:702` already uses (`document.cookie = "paraglide_lang=..."`) - no backend change needed since it's unauthenticated, cookie-only.
- **UX note from the user - now in scope for this phase:** it's hard to get to `/welcome` from the logged-in dashboard (no nav link back to it), and the "sign up a new group" link on `/welcome` is too easy to miss. Both are navigation/discoverability tweaks to `+layout.svelte` / `welcome/+page.svelte`, not architectural. To be scoped and implemented alongside group switching (see §3) since both touch the top bar.

## 2. Group signup (`/join`)

- Route behind ScoutID login (auth-gated, not in `isPublicPath`).
- **Auth constraint (found during implementation planning):** this route's whole audience is users whose org has *no* matching `groups` row yet - the exact case where `/api/v0/me` returns nothing and the standard `auth.Middleware` (auth.go:277) hard-rejects with `403 group_not_found` before any handler runs, because it requires resolving a DB group to build `Claims`. Resolution: a small, additive change to the existing middleware rather than a separate auth path:
  - Add `AllowUnmapped bool` to `MiddlewareConfig`, default `false`. Every existing mount point is unaffected.
  - When `true` and no `memberships.groups` key matches a registered group, skip the `403` and instead build `Claims{GroupID: "", Teams: nil, MaxAccess: AccessView}` plus a new field `Claims.Orgs []OrgMembership{ID, Name string; Roles []string; IsPrimary bool}` populated directly from the raw `memberships.groups` claim (no DB resolution needed - there's nothing to resolve yet). Confirmed claim shape (corrected from an earlier wrong assumption): `memberships.groups["766"] = {"name": "Scoutkåren Mälarscouterna", "roles": [{"id":136,"key":"it_manager","name":"IT Manager"}, ...], "is_primary": true}` - the org name *is* present per-org, not just per-troop.
  - Mount a second instance of `auth.Middleware` with `AllowUnmapped: true` only on the `/join` submit route's router group; every other route keeps the existing strict instance untouched.
  - The frontend cannot rely on `data.user` (it's `null` for this audience) or `/api/v0/me` either. It reads org id/name/role claims from the raw JWT the same way `+layout.server.ts`'s `extractNameFromToken` already does for `oidcName`, to prefill the form before submit.
  - The `/join` handler reads `claims.Orgs` (org ids/names/role names) instead of `claims.Teams`, since there's no team/group in the DB yet.
- **Status: implemented.** Final form fields, all required unless noted:
  - Scout group applying for: org id, shown as a dropdown only when the JWT has more than one org (defaulting to `is_primary: true`); group name is a separate editable text field prefilled from `memberships.groups[orgId].name` (e.g. applicant shortens "Scoutkåren Mälarscouterna" to "Mälarscouterna" - the claim value is just a default).
  - Scoutnet role ("Scoutnet-roll för kårens scoutrustning-admin"): dropdown of the applicant's actual roles for the selected org (`memberships.groups[orgId].roles[]`, each shown as `Name (id)`), falling back to free text only when the JWT has no roles for that org (e.g. dev-mode persona without a real JWT). Required. Help text notes more managers can be added later in group settings.
  - Manager team display name (e.g. "Utrustningsgruppen"): separate editable field from the Scoutnet role - this is what becomes the `teams.name` row via `init-group --team-name`, distinct from the person's Scoutnet role title.
  - Contact email, with help text nudging toward a shared group mailbox rather than the applicant's personal address (deliberately **not** prefilled from the applicant's own email).
  - Approximate scout group size (free text).
  - Custom domain interest: a checkbox ("Vi är intresserade av att ha en egen domän...") plus a short note that Scoutrustning is cheap to run so no fee is expected, with a possible future follow-up about server costs. No domain name is collected here - that's a follow-up email conversation, deliberately not connected to `init-group`.
  - ToS/GDPR checkbox, with the pricing note placed directly under it.
  - A small read-only note above the form showing the applicant's own name/email (from the JWT), so they know that's captured too - not editable.
- On submit: sends two emails via `internal/notifications` (`SMTPNotifier`; corrected from the plan's earlier `internal/notify` reference):
  - To `ADMIN_EMAIL` (env var, wired through `docker-compose.yml`'s `api` service `environment:` block) with the full form answers plus a ready-to-run `init-group` invocation: `go run ./cmd/server init-group --group-id=<org id> --group-name="<name>" --role-key=<role key> --team-name="<team name>"`. **Signup is refused outright (`503`) if `ADMIN_EMAIL` is unset** - there is no silent "applicant-only" fallback, since a signup nobody sees is worse than a clear error.
  - To the applicant's given contact email, a confirmation including a summary of everything they submitted (group, org id, role, contact email, group size, domain interest) - no mention of custom domain follow-up specifics, that happens over the email thread directly.
  - Both sent with `Message.GroupID: ""` - `SMTPNotifier.resolveConfig` (smtp.go:78) falls back to system `SMTP_DEFAULT_*` env vars when there's no group row.
- Endpoint: `POST /api/v0/join`, mounted as its own `r.Route` in `main.go` (sibling to the existing `/api/v0` group, not nested under it) with its own middleware stack: `auth.Middleware(..., AllowUnmapped: true)` only - explicitly *not* `handler.UpsertUserMiddleware`, since that would try to upsert a `users` row with an empty `group_id`, which violates the `NOT NULL REFERENCES groups(id)` constraint.
- `claims.Orgs` (org id/name/roles, each role carrying its Scoutnet `id`/`key`/`name`) is now populated on every authenticated request, not just the unmapped-fallback path, so a user with one registered org can still apply for a second, unregistered one via `/join`.
- Request body: `{ org_id, group_name, role_name, role_key, team_name, contact_email, group_size, interested_in_custom_domain }`. `org_id` is cross-checked server-side against `claims.Orgs` (not trusted from the client body directly) to prevent spoofing an org the user doesn't belong to.
- Entry point on `/welcome`: a bordered CTA card (same visual weight as the demo/prod try-it links, not visually emphasized above them) placed below those links, linking to `/join`.
- Provisioning stays manual for now: no signup-request table, no approve-from-email button. The security surface of an unauthenticated action link that mutates state (creates a group) isn't worth it for a rare, low-volume workflow, and group creation already happens via docker/`init-group` CLI commands.
  - Possible future iteration: an approve button/link, if we ever want it, would need a signed/expiring token and a `group_signup_requests` table — deliberately deferred, not part of this plan.

## 3. Group switching

- For users belonging to multiple registered scout groups.

### Data model change (blocking prerequisite, needs migration)

- **Problem found during planning:** `users.id` is currently the sole primary key (the Keycloak member ID), with a single required `group_id` column per row - see `.amazonq/rules/project-context.md`. This makes it physically impossible for one person to have more than one registered-group profile today: a second group's row for the same member ID would collide on the PK.
- **Resolution: composite primary key.** Change `users` PK from `id` to `(id, group_id)`. One row per (member, group) - i.e. one profile per group the person belongs to, each with its own `team_ids`, `max_access_level`, `language`, `notification_prefs`, etc.
  - Every table that currently has `user_id (uuid/text) REFERENCES users(id)` already carries its own `group_id NOT NULL` column (multi-tenancy rule). Those FKs change from `REFERENCES users(id)` to a composite FK `(user_id, group_id) REFERENCES users(id, group_id)` - mechanical, not a new modeling concept, since the `group_id` column is already present on every affected row.
  - Tables affected (need to check for `user_id`-shaped FKs to `users.id`): `bookings`, `comments`, and any others found during implementation - to be enumerated precisely in the migration, not guessed here.
  - This also resolves an ambiguity in §5 (account removal) for multi-group members - see the updated §5 below.
- Upsert-from-claims logic (`UpsertUserMiddleware` / equivalent) changes from "find user by member ID" to "find user by (member ID, group ID)" - a member logging into a second registered group creates a second `users` row rather than being blocked or overwriting the first group's profile.
- The existing unused `active_group_id text REFERENCES groups(id)` column on `users` (present since `00001_init.sql`, never wired up) is repurposed to store which group's profile is "active" for a member with more than one - see below.

### Switcher UX

- This is a rare, edge-case action (most members belong to exactly one group) - it should not take up permanent top-bar real estate.
- There is no existing group logo in the top bar today (`+layout.svelte` currently shows only the app logo, breadcrumbs, and `DevPersonaSwitcher`) - the profile picture is a new addition, not a replacement.
- Top bar shows the member's **`UserAvatar`** (existing component, `web/src/lib/components/UserAvatar.svelte`), placed at the far right of the existing right-hand cluster: `breadcrumbs → DevPersonaSwitcher → profile avatar`. Keeps it clear of the persona switcher rather than overlapping it.
- Clicking the avatar opens the **existing `UserInfoCard.svelte`** popup for the logged-in member's own user ID - reused as-is rather than building a new popup component. This supersedes the older backlog note (`docs/pre-release.md`, "own-profile avatar" item) which had planned navigating to `/profile` instead specifically to avoid the read-only card; that's now the intended entry point instead.
- The group switcher lives **inside that reused `UserInfoCard` popup**, not inline in the bar - only rendered at all when the member has more than one `(id, group_id)` row. `UserInfoCard` currently only supports viewing another user read-only (name, picture, notification email, teams, bookings) via `GET /api/v0/users/{id}` - the switcher is a new, conditionally-rendered section, gated on "is this my own card" (a `viewingOwnProfile` prop or comparing `userId` to the logged-in user) since other users' cards shouldn't show a control to switch *your* active group.
- Same own-profile-only gating adds two more actions to the card: a **settings link** (to `/profile`, where language/notification prefs/logout already partly live today) and a **sign-out link/button**. None of these three (settings, sign-out, group switcher) should appear when the card is opened for someone else's user ID.
- Additionally, the **dashboard root (`/`)** can surface the switcher (or a "switch group" affordance) directly, since that's the natural place a member would look when they want to change context.
- Active group is persisted via the `active_group_id` column on the member's row(s) (repurposed, see above) rather than a cookie - since identity is now genuinely multi-row server-side data, not just a client-side override like `dev-persona`. Switching updates `active_group_id` via a small endpoint; `hooks.server.ts` / `+layout.server.ts` read it to decide which of the member's `(id, group_id)` rows to resolve as `/api/v0/me`.
- `/api/v0/me` needs to return all of the member's registered-group rows (id, name, logo), not just the active one, so the frontend can render the switcher.
- One dev persona will be set up as belonging to multiple groups, to exercise switching locally without a real multi-group ScoutID account.

### Nav discoverability fixes (bundled into this phase - see §1)

- Add a way back to `/welcome` from the logged-in top bar (`+layout.svelte`).
- Make the "sign up a new group" link on `/welcome` more prominent.

### Implementation plan (commit order)

Each step is its own commit, reviewed/tested before moving to the next. The two purely-frontend, no-backend-dependency steps go first since they're independent and low-risk; the migration is the highest-risk, hardest-to-reverse piece and gets its own isolated commit once those land.

1. **Frontend: own-profile avatar wired to existing `UserInfoCard`.** Add `UserAvatar` to the top bar's right-hand cluster (`breadcrumbs → DevPersonaSwitcher → profile avatar`); clicking opens the existing `UserInfoCard.svelte` popup for the logged-in member's own user ID (reusing the component built for viewing other users, not a new one). No group switcher yet - there's nothing to switch between until the backend work below lands. Also closes out the `docs/pre-release.md` "own-profile avatar" backlog item. **Done.**
2. **Frontend: nav discoverability fixes (§1).** ~~Link back to `/welcome` from the logged-in top bar~~ - decided against a top-bar link (too much real estate for a rare action); instead added an "Om Scoutrustning" / "About Scoutrustning" CTA button alongside the existing dashboard buttons (`page_home_btn_about`, `+page.svelte`), linking to `/welcome`. Tried visually emphasizing the `/join` CTA on `/welcome` (green-bordered card) but reverted - the existing neutral-styled link is visible enough as-is, matching the demo/prod links. Independent of everything else in this phase. **Done.**
3. **Migration: composite `users` PK.** `goose` migration ([00016_users_composite_pk.sql](../api/migrations/00016_users_composite_pk.sql)) changing `users` PK from `id` to `(id, group_id)`; every FK previously `REFERENCES users(id)` (queried live from `pg_constraint`, not guessed from the doc: `product_images.uploaded_by`, `packages.owner_id`, `bookings.created_by`, `booking_events.actor_id`, `article_events.actor_id`, `audit_log.user_id`, `issue_reports.reporter_id`, `issue_assignees.user_id`, `issue_events.actor_id`) widened to composite `(col, group_id) REFERENCES users(id, group_id)`. `notification_log.user_id` needed no change - an earlier migration (00008) had already dropped its FK. No data backfill - a pre-migration validation query confirmed zero existing rows had a group_id mismatch between referencing table and referenced user. sqlc regeneration produced no model fallout. One real behavior dependency found and fixed: `UpsertUser`'s `ON CONFLICT (id)` no longer matched any constraint post-migration, changed to `ON CONFLICT (id, group_id)` - which is exactly the future-facing behavior step 4 needs (a second group's login inserts a new row rather than colliding). Full integration suite + smoke test pass unmodified otherwise. **Done.**
4. **Backend: multi-group upsert + claims.** Change `UpsertUserMiddleware` (or wherever user upsert happens) from "find/create by member ID" to "find/create by (member ID, group ID)" - a second registered group for the same member creates a second row instead of colliding or overwriting. Add the `active_group_id` read/write path: on login, if unset, default it to the just-resolved group; expose it in whatever struct backs `/api/v0/me`. Integration tests: member with two group rows logs into each; member with one row is unaffected (regression coverage).
5. **Backend: `/api/v0/me` multi-group response + switch endpoint.** Extend `/api/v0/me` to include the member's full list of registered groups (id, name, logo) alongside the currently-active one. Add a small endpoint (e.g. `PUT /api/v0/me/active-group`) that updates `active_group_id`, validated against the member's actual `(id, group_id)` rows (no switching into a group they don't belong to). Integration tests for both the list and the switch, including the rejection case.
6. **Frontend: own-profile actions in `UserInfoCard` + dashboard-root affordance.** When the card is opened for your own user ID (not someone else's), add: a **settings link** to `/profile`; a **sign-out** button/form reusing the existing `POST /auth/signout` pattern already on `profile/+page.svelte:1140`; and the **group switcher** (only rendered when the member has more than one `(id, group_id)` row), wired to the step-5 endpoint. Also surface the same switcher (or a link into the popup) from `/` for discoverability. `+layout.server.ts` reads the resolved active group same as it reads `data.user` today.
7. **Dev tooling: multi-group persona + smoke test.** Add a `dev-personas.json` entry belonging to two groups to exercise switching locally; add a `smoke-test.sh` check if a new route was introduced in step 5.

## 4. GDPR information page (`/gdpr`)

- Public route (or at least reachable pre-login), added to `isPublicPath()`.
- Content is deployment-specific and must not be hardcoded in source. Plan:
  - Provide a template file (e.g. `docs/gdpr.template.md`) as an example/reference, committed to the repo.
  - At runtime, the actual content is read from a deployment-provided file (e.g. `gdpr.md`, path configurable via env var, mounted into the container / read from a volume). Not committed, not hardcoded.
  - The page renders that markdown file. If the file is missing, show a clear "not configured" message rather than failing.
- No explicit accept flow. Acceptance is implicit through continued use of the service, not a checkbox or tracked acceptance record — no new `users` column, no versioning/re-acceptance logic needed.
- The page itself should state this plainly (e.g. "by using this service you agree to the handling described below"), since implicit consent is weaker without saying so somewhere.
- Checklist of what the GDPR content itself should cover (for the template):
  - What personal data is collected, named concretely: name, email, member ID, team/role, booking history, issue reports, comments - not just "personal data".
  - Purpose of processing (equipment booking, issue tracking).
  - Legal basis.
  - Where data is stored (server location, hosting provider).
  - Who operates/has access to the data (the scout group / deployment operator, not a third party, unless applicable) - this is the one field every deployment must fill in, should be the most prominent required blank in the template, not buried at the bottom.
  - Retention period and deletion process, in concrete terms: account removal is user-initiated from personal settings, and what "orphaning" means in practice (e.g. "removing your account deletes your profile; your team's shared booking history remains, no longer linked to your name").
  - User rights (access, correction, deletion) and how to exercise them.
  - Contact for data protection questions.
  - Last-updated date / version marker.
- Keep it short and scannable - bullet points, not legal prose. Scout leaders are the audience, not lawyers.

## 5. Account removal

- Self-service removal from the personal settings page, behind a confirmation step.
- Deletes the user's own profile but orphans rather than deletes their contributions:
  - Bookings *created by* the user are orphaned (kept, no longer linked to a profile) - team/shared booking history must survive. "Personal bookings" was clarified to mean bookings created by the removed user, not a separate booking category.
  - Comments they authored remain but lose the profile link (author becomes "missing"/deleted user placeholder).
- We keep storing the member ID itself (not a hash - simpler, and it's already the primary identifier used elsewhere per `users.id`). This lets us recreate the user cleanly and re-link their orphaned bookings/comments automatically the next time they log in with the same member ID, without a separate re-linking flow.
- **Updated per §3's composite PK:** removal deletes only the member's `(id, group_id)` row for the group they're removing themselves from - other groups' profiles for the same member ID, if any, are untouched. "Your account" in the confirmation UI means "your profile in this group," not a single global identity, now that one member ID can have multiple group profiles.
- Reversibility caveat: since `users.id` *is* the Keycloak member ID directly (no separate internal UUID - see `.amazonq/rules/project-context.md`), "removal" deletes that row outright. It is not an admin-reversible undo - the only way back is the same person logging in again (to that same group), which recreates their profile and re-attaches their orphaned history for that group. Framing for the confirmation UI: "this removes your profile in [group] now; if you log in again later, your history will be restored" rather than "this is permanent."
- Deferred idea, not part of this plan: automatic removal of accounts inactive for some period (e.g. 14 months), using the same orphan-and-recreate-on-login mechanism. Would need a scheduled job (none exists in this codebase yet) and a policy decision (fixed vs. per-deployment configurable). Revisit later if wanted.
- Open questions:
  - Exact DB mechanics: `bookings`/`comments` likely have `NOT NULL` FKs to `users.id` today - orphaning requires either a nullable FK or a placeholder "deleted user" row. This is a migration and needs explicit approval before implementing.

## Suggested implementation order

1. Landing page (`/welcome`) + `/login` removal — self-contained, unblocks the others since group signup and GDPR pages link from here. **Done.**
2. GDPR page (template + runtime file loading) — needed before signup ToS can link to something real. **Done.**
3. Group signup form + email. **Done.**
4. Group switching (largest scope: `users` composite-PK migration, FK updates, `/api/v0/me` multi-group response, active-group switcher endpoint, UI switcher) + bundled nav discoverability fixes (§1). **Current phase - step 3 of 7 done, step 4 (multi-group upsert + active_group_id) next.**
5. Account removal — sequenced after group switching since its scoping (§5) now depends on the composite-PK model landing first.

Each phase should be scoped, approved, and implemented separately rather than all at once.
