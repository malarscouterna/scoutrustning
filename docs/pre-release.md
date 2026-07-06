# Pre-release checklist

Work to complete before moving from `/api/v0/` (pre-release) to v1.0.

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

## Other frontend gaps

- [ ] Web header logo - fetch `logo_url` from group settings and render in top nav when present

---

## Booking flow correctness

### Self-conflict on edit

When changing dates, title, or unit on an existing booking, the availability check incorrectly flags the booking's own items as conflicts because the `ExcludingBooking` path is not applied consistently during updates. Needs a fix and tests covering: date change with no conflict, date change with a real conflict from another booking, and title/unit change that should never trigger availability errors.

Done in `fix(api): apply ExcludingBooking consistently on booking date change`:
- Root cause was not a missing call site but the query itself: `AvailableArticlesExcludingBooking` had a second exclusion clause that filtered out articles already in the booking being edited - correct for `AddItems`/`SwapItem` (offering *new* items), wrong when `Update` reused it to revalidate the booking's *existing* items against new dates.
- Fixed by adding an `exclude_own_items` boolean to the single query rather than duplicating it: `false` for `Update`'s own-item revalidation, `true` everywhere items are offered to add (`AddItems`, `SwapItem`, the `articles.go` add-item picker endpoint - the last one was missed on the first pass and caught by the full test suite).
- Bookings have no `title` field, so that part of the description didn't apply; covered date-change-no-conflict, date-change-real-conflict, and unit-change-no-availability-check instead.

### Copy booking flow

The copy API endpoint (`POST /api/v0/bookings/{id}/copy`) exists but the UI is not exposed. A user should be able to copy any existing booking to create a new one with the same items, then set new dates.

When the UI is built, the user must be able to set the new date range before conflict-checking fires - otherwise the copied items are immediately flagged as conflicting with the source booking. Items that are unavailable for the new dates should be marked, not silently included.

Tests needed: copy to clear dates, copy to dates that overlap the source, copy where some items are unavailable.

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

**3. Approval action area** - shown below the thread, contextual per `(status x role)`:
- `draft` (user): submit button + optional message field. The "Vill ha bekräftelse från ansvarig" checkbox is shown but auto-checked and non-interactive (with hover tooltip explaining why) when any item in the booking requires approval. When no item requires approval, the checkbox is optional.
- `submitted` (manager): approve/reject buttons + optional message field.
- `rejected` (user): the booking re-enters a draft-like editable state. The user can edit items and dates, add a comment, and resubmit. It should be visually clear that the items are still on hold and will be released if not resubmitted within the auto-archive window (see below).
- `approved`/`confirmed` (user): no approval actions, just the pickup button.
- Terminal states (`returned`, `cancelled`): read-only thread, no action area.

The comment thread should be shown on the booking list card as a preview (last comment, unread indicator) to make it clear that the booking is a living conversation, not just a status.

**Editing after submission:** A booking can be edited (items, dates) while in `submitted` state without needing to withdraw it. This should be visually clear - the edit controls should remain accessible in submitted state, not hidden or greyed out. The manager sees the updated booking when they review it.

**Auto-approval visibility:** When submitting, the UI should clearly indicate whether the booking will be auto-approved (trusted team + all items at `low` approval level) or requires manager review. This can be shown as a short confirmation line near the submit button, e.g. "Bokningen godkänns automatiskt" vs "Bokningen skickas för granskning". The "Vill ha bekräftelse" checkbox (when optional) lets the user override auto-approval and request a manual review anyway.

### Booking auto-archive setting

The existing 48-hour cleanup for empty drafts (no items) stays as a hard-coded system behaviour and is unaffected by group settings.

Two separate group-level settings cover bookings with items:

- **Draft with items** - default 3 days. Timer starts when the first item is added.
- **Rejected awaiting resubmission** - default 7 days. Timer starts when the booking is rejected.

The deadline is fixed from the moment the stage is entered and does not reset on edits. This keeps the countdown predictable and honest - the user knows exactly when their items will be released regardless of what changes they make.

The booking detail page shows a countdown to the deadline - including hours and minutes - whenever the booking is in one of these timed states. The display should feel urgent enough that the user takes it seriously.

The draft countdown in particular should combine urgency with reassurance: the message should make clear that submitting is the safe move, that it is not a final commitment, and that items remain on hold either way. The goal is to push users toward submitting rather than leaving bookings to expire in draft. Something like "Skicka in din bokning för att hålla den aktiv - du kan fortfarande ändra den efteråt."

When a booking is auto-archived its items are released and the event thread records the archival clearly (not a silent disappearance). Setting 0 for either disables auto-archiving for that stage.

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

When an item is still `picked_up` at the start of the next booking's date range:

**Auto-swap:** If an equivalent article (same `commercial_name` + `location`) is available, automatically swap it into the waiting booking. Applies to both individually-tracked and quantity-tracked articles - the distinction does not affect swap eligibility. No confirmation required, no notification sent. A `swap` event is logged in the booking event thread so the change is visible but not disruptive.

The swap check runs in two situations: immediately when any user marks an item as delayed, and as a nightly job for items that are overdue but have not been explicitly marked delayed.

**Notification (no swap available):** Send to the affected next booker: which items are affected and a link to their booking page. Do not include names of the current booker in the notification.

**Booking page conflict overview:** The booking detail page for the affected booking shows a warning section listing each blocked item. The warning links to a UI that shows who currently has the item - name, unit, and expected return date. Contact info shown is the user's chosen personal notification email.

**When marking an item as delayed:** The manager marking the delay is shown the next expected user of the item (if the overlap falls within the delayed window) so they can anticipate the conflict before it occurs.

---

## User info card component

Several flows need to show information about another user (current booker, person who picked up an item, collaborator on a booking). A shared component is needed rather than ad-hoc name/contact rendering.

**Full card (popup/sheet):** Name, profile picture, personal notification email, list of team affiliations with type and access level. If shown in the context of a specific booking, highlight the affiliation relevant to that booking and list other open bookings. Used on booking detail (show current holder of a delayed item), issue detail, and collaborator management.

**Compact view (selector/inline):** Name + profile picture only. Used in dropdowns, the collaborator list on a booking, and anywhere a user reference appears inline.

**Profile pictures:** Sourced from the Keycloak OIDC `picture` claim, stored alongside other user claims on login. Initials-based avatar as fallback when the claim is absent.

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
6. `feat(api,web): user info card component - full card and compact view` - Shared component needed by 10 and 11. Uses Keycloak `picture` claim with initials fallback.
7. `feat(api,web): booking comment thread and approval flow redesign` - Unified event/comment thread, structured approval events, always-checked confirmation when item requires approval. Depends on 1.
8. `feat(api,web): booking auto-archive setting` - Group setting, cleanup job, advance notifications. Depends on 7.
9. `feat(api,web): copy booking UI` - Expose existing API endpoint, date-first flow, unavailable items marked. Independent.
10. `feat(api,web): delayed return - auto-swap, conflict overview, next-booker notification` - Auto-swap logic, booking page conflict section, notification without names. Depends on 6.
11. `feat(api,web): collaborative bookings - add enheter and people to a booking` - New participants model, shared pickup rights. Depends on 6 and 7.
12. `feat(web): web header logo` - Fetch logo_url from group settings, render in top nav. Independent, can go anywhere.
13. `feat(api,web): per-item descriptions for individually-tracked articles` - New `description` column on `articles`, edit field in manager article view, display on pickup checklist. Independent.
14. `feat(web): free-form image crop in issue reporting` - Replace locked-ratio crop with free-form crop in the issue reporting upload flow. Independent.

---

## Release gate

- [ ] All items above complete
- [ ] Smoke tests pass on a clean `docker compose up` + seed
- [ ] API moved from `/api/v0/` to `/api/v1/`
- [ ] `init-group` run on production VPS
- [ ] Release Please PR merged → v1.0.0 tag + Docker images pushed
