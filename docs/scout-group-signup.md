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
- **UX note from the user, explicitly deferred (do not fix without asking first):** it's hard to get to `/welcome` from the logged-in dashboard (no nav link back to it), and the "sign up a new group" link on `/welcome` is too easy to miss. Both are navigation/discoverability tweaks to `+layout.svelte` / `welcome/+page.svelte`, not architectural - revisit when asked.

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
- Landing area (likely the top bar / `+layout.svelte`) shows the currently selected group: logo (fallback to group name if no logo).
- If the user belongs to more than one registered group, expose a switcher (dropdown or similar) to change the active group.
- Active group is stored in a cookie, following the existing `dev-persona` cookie pattern — consistent with how identity/role overrides already work, avoids a DB column and update endpoint just for this, and survives page loads without extra derivation logic.
- `/api/v0/me` needs to return all of a user's registered groups, not just the current one, so the frontend can render the switcher.
- One dev persona will be set up as belonging to multiple groups, to exercise switching locally without a real multi-group ScoutID account.

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
- Reversibility caveat: since `users.id` *is* the Keycloak member ID directly (no separate internal UUID - see `.amazonq/rules/project-context.md`), "removal" deletes that row outright. It is not an admin-reversible undo - the only way back is the same person logging in again, which recreates their profile and re-attaches their orphaned history. Framing for the confirmation UI: "this removes your profile now; if you log in again later, your history will be restored" rather than "this is permanent."
- Deferred idea, not part of this plan: automatic removal of accounts inactive for some period (e.g. 14 months), using the same orphan-and-recreate-on-login mechanism. Would need a scheduled job (none exists in this codebase yet) and a policy decision (fixed vs. per-deployment configurable). Revisit later if wanted.
- Open questions:
  - Exact DB mechanics: `bookings`/`comments` likely have `NOT NULL` FKs to `users.id` today - orphaning requires either a nullable FK or a placeholder "deleted user" row. This is a migration and needs explicit approval before implementing.

## Suggested implementation order

1. Landing page (`/welcome`) + `/login` removal — self-contained, unblocks the others since group signup and GDPR pages link from here.
2. GDPR page (template + runtime file loading) — needed before signup ToS can link to something real.
3. Group signup form + email.
4. Account removal.
5. Group switching (largest scope: touches `/api/v0/me`, session/active-group concept, UI switcher).

Each phase should be scoped, approved, and implemented separately rather than all at once.
