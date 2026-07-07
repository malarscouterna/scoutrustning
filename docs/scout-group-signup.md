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

## 2. Group signup (`/join`)

- Route behind ScoutID login (auth-gated, not in `isPublicPath`).
- A form, prefilled from the logged-in user's claims/teams:
  - Scout group applying for: group id + group name, auto-populated from the user's OIDC org claim.
  - Manager team: if the applier belongs to multiple teams, they choose from a dropdown (also applies to scout group selection if they belong to multiple registered groups).
  - Contact email for the manager team.
  - Approximate scout group size (number of members).
  - Optional custom domain: free-text field for the domain they'd like (e.g. `bokning.mingrupp.se`), with explanatory copy that this is optional, costs a small amount (max a few hundred kr/year, to cover server costs), is arranged case-by-case, and that they'll be contacted about it if relevant. Normal use on the shared `scoutrustning.se` domain stays free.
  - Simple terms-of-service checkbox + link to GDPR info (`/gdpr`).
- On submit: sends two emails, both via the existing `internal/notify` email pipeline:
  - To the global admin address (new env var, e.g. `ADMIN_EMAIL` or `GLOBAL_ADMIN_EMAIL` in `.env`) with the full form answers, including a ready-to-run `init-group` CLI invocation pre-filled with the applicant's data (so the admin copy-pastes rather than re-typing it from scratch).
  - To the applicant's given contact email, a welcome/confirmation email with general information about next steps. Further back-and-forth (including any custom-domain discussion) continues on that email thread directly between the admin and the group, outside the app.
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
