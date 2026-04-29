# Notifications

Email notifications for booking lifecycle events, issue reports, and reminders. The architecture supports additional channels (e.g. chat tools) per group — only email is implemented in Phase 3.

## SMTP configuration

Uses [wneessen/go-mail](https://github.com/wneessen/go-mail) for sending. One-shot `DialAndSend` per notification (no persistent connection — volume does not justify the complexity). Dial timeout: 10 s.

SMTP credentials are configured at two levels:

### System-wide default

A fallback SMTP configuration for groups that have not set their own. Stored as environment variables (not in the database):

```
SMTP_DEFAULT_FROM=noreply@example.com
SMTP_DEFAULT_HOST=smtp.example.com
SMTP_DEFAULT_PORT=587
SMTP_DEFAULT_TLS=starttls          # starttls | tls (implicit). Default: starttls
SMTP_DEFAULT_USER=apikey
SMTP_DEFAULT_KEY=<plaintext, never logged>
```

Auth method: PLAIN (covers Gmail app passwords, SendGrid, Mailgun, Brevo, and similar providers). Not configurable — no realistic use case for alternatives.

`SETTINGS_ENCRYPTION_KEY` (already required for other encrypted fields) is also used here. **`gen-env.sh` must be updated to include both `SETTINGS_ENCRYPTION_KEY` and the `SMTP_DEFAULT_*` vars** (with `CHANGEME` placeholders for demo/prod modes; omitted in dev mode since SMTP is a no-op there).

The system-wide default is managed by the server operator, not by any scout group.

### Per-group override

Each group can configure their own SMTP sender in group settings. All fields except `smtp_key_encrypted` are stored in plain text — only the password is sensitive.

| Field | Stored as | Notes |
|---|---|---|
| `notification_email_from` | plain text | e.g. `utrustning@malarscouterna.se` |
| `smtp_host` | plain text | e.g. `smtp.gmail.com` |
| `smtp_port` | integer | e.g. `587` |
| `smtp_tls` | plain text | `starttls` \| `tls`. Defaults to `starttls` if unset |
| `smtp_user` | plain text | e.g. `apikey` (SendGrid) or a Gmail address |
| `smtp_key_encrypted` | AES-256-GCM encrypted bytes | Same `crypto.Encrypt`/`crypto.Decrypt` pattern, keyed by `SETTINGS_ENCRYPTION_KEY` |

When sending, the API checks for a per-group SMTP key first. If none is set, it falls back to the system-wide env config. If neither is configured, notification sending fails gracefully (logged, not returned as an error to the caller).

The `smtp_key_encrypted` field is never returned in API responses. The group settings endpoint returns `smtp_key_set: bool` and `smtp_key_masked: string` (first 3 + last 4 chars), exactly as today. All other per-group SMTP fields are returned as-is.

## Notification events

Each event has a recipient class (who gets it) and a trigger condition.

### Booking events

| Event key | Trigger | Default recipients |
|---|---|---|
| `booking_needs_approval` | Booking submitted and ≥1 item requires approval, or `force_approval` set | Managers (off by default for non-managers) |
| `booking_submitted_no_approval` | Booking submitted and auto-confirms (no approval needed) | Opt-in only — managers who want full visibility |
| `booking_confirmed` | Status transitions to `confirmed` (auto or after approval) | Booking creator + used-by team members |
| `booking_rejected` | Manager rejects booking | Booking creator |
| `booking_cancelled` | Booking cancelled by any party | Booking creator + used-by team members |
| `booking_reminder` | 1 day before `start_date` for `confirmed`/`picked_up` bookings | Booking creator + used-by team members |
| `booking_overdue` | `end_date` has passed and booking is still `picked_up` | Booking creator + team members + managers (opt-in) |
| `booking_any_created` | Any booking created (draft or submitted) | Opt-in only — managers who want full group visibility |

### Issue events

| Event key | Trigger | Default recipients |
|---|---|---|
| `issue_created` | Any new issue report | Managers (opt-in for others) |
| `issue_assigned_to_me` | User added as assignee | That user |
| `issue_resolved` | Status transitions to `resolved` | Reporter + all assignees |
| `issue_commented` | New `comment` event on the issue | Reporter + all assignees |

## Per-user notification preferences

Preferences are stored per **(user, group, event_type, channel)**. This means:
- A user can enable email but not another channel for `booking_confirmed`, or vice versa.
- Adding a new channel does not require changing the table structure.
- A user in multiple groups has independent preferences per group.

Missing rows fall back to the **group default**, which falls back to the **system default** (hardcoded). "Restore to defaults" deletes all rows for `(user_id, group_id)`.

### System defaults (hardcoded)

| Event key | email default (non-manager) | email default (manager) |
|---|---|---|
| `booking_needs_approval` | `false` | `true` |
| `booking_submitted_no_approval` | `false` | `false` |
| `booking_confirmed` | `true` | `true` |
| `booking_rejected` | `true` | `true` |
| `booking_cancelled` | `true` | `true` |
| `booking_reminder` | `true` | `true` |
| `booking_overdue` | `true` | `true` |
| `booking_any_created` | `false` | `false` |
| `issue_created` | `false` | `true` |
| `issue_assigned_to_me` | `true` | `true` |
| `issue_resolved` | `true` | `true` |
| `issue_commented` | `true` | `true` |

### Group-configurable defaults

Equipment managers can set group-level defaults in group settings. These override the system defaults for all users in the group who have not set their own preference for that `(event_type, channel)`. Stored in `group_notification_defaults`.

Use case: a group decides all members should receive `booking_reminder` via email — they set the group default to `true`. Individual users can still override it.

### API

```
GET    /api/v0/me/notification-prefs
PUT    /api/v0/me/notification-prefs
DELETE /api/v0/me/notification-prefs

GET    /api/v0/group-settings/notification-defaults   (manager only)
PUT    /api/v0/group-settings/notification-defaults   (manager only)
```

`GET /me/notification-prefs` returns the *effective* (merged) value per event+channel, plus a flag indicating whether it was user-set or inherited:

```json
{
  "prefs": {
    "booking_confirmed":   { "email": { "enabled": true,  "source": "user" } },
    "issue_created":       { "email": { "enabled": false, "source": "group_default" } },
    "booking_any_created": { "email": { "enabled": false, "source": "system_default" } }
  }
}
```

`PUT /me/notification-prefs` is a partial update — only keys present are changed:

```json
{
  "booking_confirmed": { "email": true },
  "issue_created":     { "email": true }
}
```

`DELETE /me/notification-prefs` removes all user-level rows for the active group, reverting everything to group/system defaults.

The group defaults endpoints use the same shapes, without the `source` field.

The existing `GET /api/v0/group-settings` response gains a `notification_channels` field — the ordered list of channel identifiers active for this group (e.g. `["email"]`). The frontend uses this to determine which columns to render in the preference table. Managed by the server operator for now (derived from which `Notifier` implementations are wired up); not editable by group managers in Phase 3.

The frontend shows these on the profile page (`/profile`) under a "Notiser" section. Layout: a semantic `<table>` where rows are notification event types and columns are channels. Event types are grouped into Bokningar and Ärenden using `<tbody>` sections with a spanning group-header row.

**Column rendering**: columns are driven by `notification_channels` from the group settings response (e.g. `["email"]`). Only configured channels appear — no placeholder columns for future channels. When a group adds a new channel, it appears automatically.

**Row behaviour**:
- Rows irrelevant to the user's role (`booking_needs_approval`, `issue_created` for non-managers) are absent, not grayed out.
- `issue_assigned_to_me` is rendered as a non-toggle informational row (lock icon, tooltip: "Du får alltid notiser när du tilldelas ett ärende").
- All other rows show a toggle per channel.

**Inheritance hint**: the `source` field drives a small "(gruppstandard)" or "(systemstandard)" label under the event name when the value is not user-set. One hint per row — all channels in that row share the same source.

A "Återställ till standard" button calls `DELETE /me/notification-prefs` and reverts all rows to group/system defaults.

## Issue assignees

Issues can be assigned to one or more users via the `issue_assignees` table (already in schema). Assignment is done by managers from the issue detail page.

### Group members API

Required for the assignee picker. Returns all users who have ever logged in to the group.

```
GET /api/v0/users?min_access_level=trusted   (manager only)
```

Query params:
- `access_levels` — comma-separated list of access levels to include, e.g. `trusted,manager`. Filters to users whose highest team access level is one of the listed values. Default: no filter (returns all). The assignee picker passes `trusted,manager` so that only managers and trusted users appear — keeping responses light for large groups and the list meaningful for assignment. Other callers (e.g. a future "Medlemmar" settings page) can omit the param or pass a different set.

Response: `[{ "id": "string", "name": "string", "email": "string", "access_level": "trusted" }]`

`access_level` is the user's highest effective access level in the group (derived from their team memberships). This is shown in the assignee picker UI so managers can see at a glance who they're assigning to.

In demo mode (`DEMO_MODE=true`), only personas from `dev-personas.json` are returned — real user records are filtered out to prevent exposing personal details. Personas are seeded into the DB (via `dev-seed.sh`), so their access levels come from the DB as normal. In dev mode (`DEV_MODE=true`, no `DEMO_MODE`), both real logged-in users and seeded personas are returned — enabling full local testing.

### Assignee picker UI

Inline on the issue detail page. Managers see current assignees as chips with remove buttons, plus a "Tilldela" button that opens a searchable dropdown of group members. Non-managers see the assignee list read-only.

### Assignee API endpoints

```
POST   /api/v0/issues/{id}/assignees            body: { "user_id": "..." }   (manager only)
DELETE /api/v0/issues/{id}/assignees/{userId}                                (manager only)
```

Adding an assignee inserts into `issue_assignees` and creates an `issue_events` row: `event_type = "assignment"`, `metadata = { "user_id": "...", "action": "added" }`. Returns 409 if already assigned. Removal creates the same event with `"action": "removed"`.

The existing `GET /api/v0/issues/{id}` response includes `assignees: [{ "id", "name" }]`.

### Assignment notification

On insert, if the assignee's effective `issue_assigned_to_me` preference is enabled for the channel, a notification is sent. No notification on removal.

## Scheduled notifications

Two jobs run in a single daily scheduler goroutine started at server startup.

### Start reminder

- **When**: daily at `NOTIFICATION_REMINDER_TIME` (default `08:00`, in server local time / `TZ` env var)
- **What**: `confirmed`/`picked_up` bookings where `start_date = tomorrow`
- **Recipients**: creator + all members of the used-by team, filtered by effective `booking_reminder` preference

### Overdue check

- **When**: same daily run
- **What**: `picked_up` bookings where `end_date < today`
- **Recipients**: creator + team members filtered by `booking_overdue`; plus managers opted into `booking_overdue`
- **Deduplication**: send once per `(booking_id, user_id, channel)` — tracked via `notification_log`

## Data model additions

### users.notification_prefs

JSONB column on `users`. Shape: `{"booking_confirmed": {"email": true}, ...}`. Missing keys fall back to `group_settings.notification_defaults`, then system defaults. Written via `PUT /me/notification-prefs`; deleted (reset to `{}`) by `DELETE /me/notification-prefs`.

```sql
ALTER TABLE users ADD COLUMN notification_prefs jsonb NOT NULL DEFAULT '{}';
```

### users.max_access_level

Denormalized highest access level for the user, updated on every login via `UpsertUser`. Enables `GET /api/v0/users?access_levels=trusted,manager` filtering without rejoining through OIDC claim mappings.

```sql
ALTER TABLE users ADD COLUMN max_access_level text NOT NULL DEFAULT 'book';
```

### users.team_ids

Denormalized list of team UUIDs the user belongs to, updated on every login via `UpsertUser`. The auth middleware already resolves `Claims.Teams []TeamMembership` (each with a `TeamID`) at login — this column persists that resolution so notification send functions can query "all members of team X" without rejoining through OIDC claim mappings.

```sql
ALTER TABLE users ADD COLUMN team_ids uuid[] NOT NULL DEFAULT '{}';
```

Added in migration `00006_user_team_ids.sql`. `UpsertUser` is updated to accept and store `team_ids` (full replacement on every login). `UpsertUserMiddleware` extracts `TeamID` from each entry in `claims.Teams` and passes the slice.

Staleness caveat: if a user's team membership changes, `team_ids` reflects the old state until their next login. Acceptable for this system since login is a prerequisite to booking activity.

Dev seed: `dev-seed.sh` inserts users directly and must include the correct team UUIDs; otherwise seeded personas have `team_ids = '{}'` and won't receive team-scoped notifications.

New sqlc queries added alongside this:
- `GetTeamMembersWithEmails(team_id uuid, group_id text)` — `WHERE @team_id = ANY(team_ids)` — booking recipient resolution (used-by team members).
- `GetGroupManagers(group_id text)` — `WHERE max_access_level = 'manager'` — manager-recipient events (`booking_needs_approval`, `issue_created`).

### users.team_ids

Denormalized list of team UUIDs the user belongs to, updated on every login via `UpsertUser`. The auth middleware already resolves `Claims.Teams []TeamMembership` (each with a `TeamID`) — this column simply persists that resolution so notification send functions can query "all members of team X" without rejoining through OIDC claim mappings.

```sql
ALTER TABLE users ADD COLUMN team_ids uuid[] NOT NULL DEFAULT '{}';
```

Added in migration `00006_user_team_ids.sql`. `UpsertUser` is updated to accept and store `team_ids`. `UpsertUserMiddleware` extracts `TeamID` from each entry in `claims.Teams` and passes the slice.

New sqlc queries:
- `GetTeamMembersWithEmails(team_id uuid, group_id text)` — `WHERE team_id = ANY(team_ids)` — used for booking recipient resolution (creator + team members).
- `GetGroupManagers(group_id text)` — `WHERE max_access_level = 'manager'` — used for manager-recipient events (`booking_needs_approval`, `issue_created`).

### group_settings SMTP fields + notification_defaults

New plain-text SMTP fields and a JSONB notification defaults column added to `group_settings`. Same shape as `users.notification_prefs`, but keyed by `(event_type, channel)` only — applies to all users in the group who have no user-level preference.

```sql
ALTER TABLE group_settings ADD COLUMN smtp_host             text    NOT NULL DEFAULT '';
ALTER TABLE group_settings ADD COLUMN smtp_port             integer NOT NULL DEFAULT 587;
ALTER TABLE group_settings ADD COLUMN smtp_tls              text    NOT NULL DEFAULT 'starttls';
ALTER TABLE group_settings ADD COLUMN smtp_user             text    NOT NULL DEFAULT '';
ALTER TABLE group_settings ADD COLUMN notification_defaults jsonb   NOT NULL DEFAULT '{}';
```

### notification_log

Prevents duplicate sends for scheduled jobs and records delivery status.

```sql
CREATE TABLE notification_log (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    group_id    text NOT NULL REFERENCES groups(id),
    user_id     text NOT NULL REFERENCES users(id),
    event_type  text NOT NULL,
    entity_id   uuid NOT NULL,   -- booking_id or issue_id
    channel     text NOT NULL,   -- 'email'
    status      text NOT NULL,   -- 'sent', 'failed', 'skipped'
    error       text,
    created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX ON notification_log (entity_id, event_type, user_id);
```

Scheduled jobs check this table before sending: if a `sent` row exists for `(entity_id, event_type, user_id, channel)`, skip.

## Implementation notes

### Sending email

Uses `wneessen/go-mail`. One-shot `DialAndSend` per notification, 10 s dial timeout.

```go
type Notifier interface {
    Send(ctx context.Context, msg Message) error
}

type Message struct {
    GroupID string
    To      string
    Subject string
    Body    string // HTML
}
```

`GroupID` is set by `send.go` before calling `Notifier.Send` so the notifier can look up per-group SMTP config without needing a separate constructor argument.

`SMTPNotifier` resolves config per-send: per-group fields (`smtp_host`, `smtp_port`, `smtp_tls`, `smtp_user`, `crypto.Decrypt(smtp_key_encrypted)`) → system env vars (`SMTP_DEFAULT_*`). TLS mode maps to go-mail's `mail.TLSOpportunistic` (STARTTLS) or `mail.TLSMandatory` (implicit TLS). Auth is always PLAIN. `NoopNotifier` discards silently; `CapturingNotifier` (mutex-safe) records messages for tests.

### Fire and forget

Notifications fire after the primary DB write succeeds, in a goroutine. Failure is logged but never propagates to the caller.

```go
if h.Notifier != nil {
    b, n, q := updated, h.Notifier, h.Q
    go notifications.SendBookingConfirmed(context.Background(), q, n, b)
}
```

### Preference resolution

```go
func IsEnabled(ctx context.Context, q *db.Queries, userID, groupID, event, channel string, isManager bool) bool
```

Fast single-pair lookup used by send functions. Queries user prefs → group defaults → system defaults in order; returns on first match. `isManager` is derived from `user.max_access_level == "manager"`.

### Email bodies

Minimal stub HTML built inline in `send.go` using i18n keys (`email_subject_*` and `notif_*` in `api/internal/i18n/messages/sv.json` and `en.json`). Language resolves: recipient `users.language` → `sv`. The HTML wrapper is `simpleBody` in `send.go` — currently just a bare `<p>` tag with no context, greeting, or link back to the booking/issue. Bodies will be replaced with proper templates in a later pass (Step 10).

### Adding a new channel

The `channel` dimension in `user_notification_prefs` and `group_notification_defaults` makes new channels additive: add the channel identifier to `notification_channels` in group settings, implement a new `Notifier`, add rows for the new channel in the defaults tables. No schema changes needed. The preference UI picks up the new column automatically.

## Implementation plan

Each step is independently testable. Steps 1–3 are backend prerequisites. Steps 4–7 add visible functionality. Steps 8–9 are scheduled jobs and manager defaults UI.

### Step order summary

| Step | Deliverable | Status | Prerequisites |
|---|---|---|---|
| 1 | Migration | ✅ done | — |
| 2 | Group members API | ✅ done | 1 |
| 3 | Issue assignee API | ✅ done | 1, 2 |
| 4 | Assignee picker UI | ✅ done | 3 |
| 5 | Preference data layer + API | ✅ done | 1 |
| 6 | Preference UI | ✅ done | 5 |
| 6.5 | users.team_ids migration + UpsertUser update | ✅ done | — |
| 7 | Notifier + event sends | ✅ done | 1, 3, 5, 6.5 |
| 7.5 | Demo mode protection + test email button | ✅ done | 7 |
| 8 | Scheduled jobs | — | 7 |
| 9 | Group defaults UI + SMTP UI | ✅ done | 5, 6 |
| 10 | Email body templates | — | 7 |

Steps 3 and 5 can run in parallel after Step 1. Steps 4, 6, and 9 are frontend-only and can overlap with later backend steps once their APIs exist.

---

### Step 1: Migration ✅

Added in `migrations/00005_notifications.sql`:
- `users.max_access_level` — denormalized highest access level, updated on every login via `UpsertUser`
- `users.notification_prefs` — JSONB, per-user pref overrides
- `group_settings.smtp_host/port/tls/user` — plain-text SMTP fields
- `group_settings.notification_defaults` — JSONB, per-group pref overrides
- `notification_log` — deduplication table for scheduled jobs

Note: prefs are stored as JSONB on the parent rows rather than a separate `user_notification_prefs` table. This avoids extra joins for the common read path and keeps preference resolution entirely in Go code.

### Step 2: Group members API ✅

`GET /api/v0/users` — manager only. Returns all users who have logged into the group, optionally filtered by `access_levels` query param.

**Demo mode protection**: in `DEMO_MODE=true`, results are filtered to only persona IDs (read from `dev-personas.json` at startup). Real user records that accumulated in the DB are hidden. In `DEV_MODE=true` (local dev, no demo mode), both seeded personas and real logged-in developers are returned — enabling full local testing. `UserHandler` receives a `PersonaIDs map[string]bool` that is nil in non-demo mode (filter bypassed).

`GET /group-settings` now includes `notification_channels: ["email"]` — the ordered list of active channel identifiers. The frontend uses this to determine which columns to render in the preference table. Hard-wired to `["email"]` for Phase 3; adding a channel means adding a `Notifier` implementation and the identifier appears in the response automatically.

### Step 3: Issue assignee API ✅

- `POST /api/v0/issues/{id}/assignees` — manager only, returns 409 if already assigned
- `DELETE /api/v0/issues/{id}/assignees/{userId}` — manager only
- Both create an `issue_events` row with `event_type = "assignment"` and `metadata = { "user_id", "user_name", "action" }`
- `GET /api/v0/issues/{id}` includes `assignees: [{ "id", "name" }]`

`user_name` is stored in event metadata at write time so that the event log can display a name even after a user is removed or renamed.

### Step 4: Assignee picker UI ✅

Assignee section on `/issues/[id]`. Manager: chips with remove buttons + "Tilldela" button opening a searchable dropdown. Group members are fetched once on first open and filtered client-side. Access level shown in Swedish below the name. Non-manager: read-only chip list.

Assignment event log renders "tilldelade / tog bort tilldelning för [name]" rather than a generic label.

`dev-seed.sh` upserts all personas so they appear in `GET /users` without requiring a prior login in the dev environment.

### Step 5: Notification preference data layer + API ✅

**Files added:**
- `internal/notifications/prefs.go` — event key constants, `systemDefaults`, `ResolvePrefs` (full map for API responses), `IsEnabled` (fast single-pair lookup for send functions in Step 7)
- `internal/handler/notification_prefs.go` — 5 endpoints
- `internal/handler/me.go` — `MeHandler` consolidating `GET /me`, `PUT /me/language`, and `/me/notification-prefs` sub-routes (refactored out of inline closures in `main.go`)
- sqlc queries: `GetUserNotificationPrefs`, `SetUserNotificationPrefs`, `ClearUserNotificationPrefs`, `GetGroupNotificationDefaults`, `SetGroupNotificationDefaults`

**Endpoints:**
- `GET /me/notification-prefs` — merged effective prefs with `source` field (`"user"` | `"group_default"` | `"system_default"`)
- `PUT /me/notification-prefs` — partial update; only keys present are changed
- `DELETE /me/notification-prefs` — resets `notification_prefs` to `{}`, reverting to group/system defaults
- `GET /group-settings/notification-defaults` — manager only, returns raw group overrides
- `PUT /group-settings/notification-defaults` — manager only, full replacement

**Design decisions:**
- `PUT /me/notification-prefs` is a partial merge (only supplied keys change). `PUT /group-settings/notification-defaults` is a full replacement — matches the group settings page UX where the whole table is saved at once.
- `activeNotificationChannels` is a package-level var in the handler package (`["email"]`), shared between `NotificationPrefsHandler` and `GroupSettingsHandler` so `notification_channels` in the group settings response stays in sync with the columns rendered in the prefs table.

### Step 6: Notification preferences UI ✅

"Notiser" section on `/profile`. Semantic `<table>`: rows = event types grouped into Bokningar/Ärenden, columns = channels from `group_settings.notification_channels`. Columns render dynamically — no hardcoded channel names. `booking_needs_approval` and `issue_created` absent for non-managers. `issue_assigned_to_me` is a non-toggle informational row. `source` shown as hint under the event label when inherited. "Återställ till standard" calls `DELETE /me/notification-prefs`.

**Test**: SSR smoke test. Toggles and restore button work for both leader and manager personas.
TODO: when changing back to the default setting, once again indicate that we are no the default setting.

### Step 7: Notification infrastructure + event-triggered sends ✅

**Files added:**
- `api/internal/notifications/notifier.go` — `Notifier` interface, `NoopNotifier`, `CapturingNotifier` (mutex-safe, for tests)
- `api/internal/notifications/smtp.go` — `SMTPNotifier` (per-group SMTP config → env vars fallback, GroupID from `Message`)
- `api/internal/notifications/send.go` — typed send functions, language resolution, preference check via `IsEnabled`

Email bodies are minimal stub HTML built inline with i18n keys. Visual polish deferred to a later pass.

**Wired into handlers** (fire-and-forget goroutines, nil-guarded):
1. `booking_needs_approval` — submit handler, when approval is required
2. `booking_submitted_no_approval` — submit handler, auto-confirm path
3. `booking_confirmed` — approve handler + auto-confirm path
4. `booking_rejected` — reject handler
5. `booking_cancelled` — cancel handler
6. `issue_created` — create issue handler
7. `issue_assigned_to_me` — add assignee handler
8. `issue_resolved` — issue status update, `resolved` transition
9. `issue_commented` — add comment handler

**`BookingHandler`** and **`IssueHandler`** both gain a `Notifier notifications.Notifier` field. `main.go` wires `&notifications.SMTPNotifier{Q: queries}`.

**Test** (`TestNotifications_EventTriggered`): injects `CapturingNotifier`. Covers needs_approval, confirmed (after approve), rejected, cancelled, issue_created, and action-succeeds-without-SMTP. Goroutine synchronization via timed poll with `time.Sleep`.

### Step 7.5: Demo mode protection + test email ✅

**Demo mode protection:**

In `DEMO_MODE=true`, `main.go` wires `NoopNotifier` for all handler event sends. The test-email endpoint always uses `SMTPNotifier` directly regardless of mode, so demo visitors can verify SMTP config.

**Test email endpoint:**

```
POST /api/v0/me/test-email
```

- **Personas**: returns `{"skipped": true}` 200 — no send.
- **Real users**: sends via `SMTPNotifier` (per-group → env fallback). Returns `{"sent": true}` or 503 if SMTP not configured.
- **Demo mode**: sends — intentional, designated way for demo visitors to verify email.

Frontend: "Skicka testnotis" button in the Aviseringar section of the group settings tab on `/profile`. Shows success/error inline. Button and endpoint both live in `MeHandler`.

**API response:**
```json
{ "skipped": true }   // persona
{ "sent": true }      // sent
```

**Tests** (`TestNotifications_TestEmail`): real user receives email, persona is skipped, demo mode sends via injected `CapturingNotifier`.

### Step 8: Scheduled jobs

`api/internal/notifications/scheduler.go` — goroutine started in `main.go` that fires daily at `NOTIFICATION_REMINDER_TIME`.

- `SendReminders`: bookings with `start_date = tomorrow` and status `confirmed`/`picked_up`. Recipients: creator + team members. Deduplicates via `notification_log`.
- `SendOverdueAlerts`: `picked_up` bookings with `end_date < today`. Recipients: creator + team members + opted-in managers. Once-only per `(booking_id, user_id, channel)`.

**Test** (`TestNotifications_Scheduled`): call `SendReminders` and `SendOverdueAlerts` directly (not via timer). Assert correct recipients. Run twice — assert no duplicates. User with preference off — no message. Wrong date — not included.

### Step 9: Group notification defaults UI + SMTP settings UI ✅

Both live in the group settings tab of `/profile` (manager only).

**SMTP settings section ("Aviseringar"):**

A checkbox "Använd egna SMTP-inställningar" controls whether the group overrides the system SMTP. When unchecked, the system sender address (`SMTP_DEFAULT_FROM`) is shown as informational text. When checked, a form appears with: Från-adress, SMTP-server, Port, TLS-läge (STARTTLS / Implicit TLS), Användarnamn, Lösenord. Saving with the checkbox unchecked clears all per-group SMTP fields. The password field leaves the existing key unchanged when left blank (nil vs empty string pattern already in use).

`GET /api/v0/group-settings` response gains two new fields:
- `system_smtp_configured: bool` — whether `SMTP_DEFAULT_HOST` env var is set
- `system_smtp_from: string` — value of `SMTP_DEFAULT_FROM`

`PUT /api/v0/group-settings` now accepts and stores `smtp_host`, `smtp_port`, `smtp_tls`, `smtp_user` in addition to the existing fields. Implementation: `UpsertGroupSettings` (handles key + all non-smtp fields) followed by `UpdateSmtpSettings` (handles smtp_host/port/tls/user), using the existing generated queries.

`gen-env.sh` updated: `SETTINGS_ENCRYPTION_KEY` (auto-generated 256-bit hex) and `SMTP_DEFAULT_*` block appended for demo/prod modes; omitted for dev (SMTP is a no-op in dev).

**Group notification defaults section ("Standardinställningar för notiser"):**

Toggle table with the same layout as the user prefs table. `GET /api/v0/group-settings/notification-defaults` returns:
- `defaults` — effective values: system defaults merged with group overrides (what group members actually receive by default)
- `system_defaults` — raw system defaults, used by the frontend to show a "(standard)" hint when the group value matches the system default

`PUT /api/v0/group-settings/notification-defaults` stores the full incoming map as-is (no pruning — intentional group overrides are preserved even if they happen to match the system default).

**`ChannelPref` extended** (user prefs response): gains `default_enabled bool` — the group/system default value for that event+channel, independent of any user override. The user prefs table uses `enabled === default_enabled` to decide whether to show the "(standard)" hint, so the hint reappears when a user toggles back to the default value and disappears when they differ from it — regardless of whether `source` is `"user"`, `"group_default"`, or `"system_default"`.

`notifications.SystemDefaults` exported (was `systemDefaults`) so the handler package can use it for merging.

### Step 10: Email body templates

Replace `simpleBody` in `send.go` with proper HTML email templates. Each event gets a template that includes:
- A greeting (recipient name if available)
- Contextual detail (booking dates, item names, issue title, commenter name, etc.)
- A direct link back to the relevant booking or issue in the app
- Plain footer (group name, unsubscribe hint pointing to `/profile`)

**String locations**: subject lines and body copy live in `api/internal/i18n/messages/sv.json` and `en.json` under the `email_subject_*` and `notif_*` key namespaces. The HTML layout (wrapper, button styles, colors) can be a Go template file or built inline — keep it simple enough to render correctly in common mail clients without a full framework.

**Test**: extend `TestNotifications_EventTriggered` to assert that captured message bodies contain the booking/issue link and relevant context (dates, title). Visual review via Mailpit (included in dev Compose stack, `http://localhost:8025`).
