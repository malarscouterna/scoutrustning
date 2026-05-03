# Notifications — Phase 3.5

Expands the Phase 3 notification system from a two-tier (Group/User) to a three-tier (Group/Team/User) architecture, adds team broadcast channels (shared email, Google Chat Space), and lays the groundwork for future channels (Slack, Teams). Delivered in two sequential phases.

---

## Channel architecture

Channels (`"email"`, `"gchat"`, `"slack"`, …) are first-class identifiers throughout the preference and dispatch system. Adding a new channel is always additive:

1. Implement a new `Notifier`.
2. Register it in the group's `enabled_channels` list (see below).
3. No schema changes needed — pref tables and notification_log are already keyed by channel.

Managers control which channels are active for their group. Only enabled channels appear in the preference table, the team settings UI, and the dispatch loop.

### `group_settings.enabled_channels`

```sql
ALTER TABLE group_settings ADD COLUMN enabled_channels text[] NOT NULL DEFAULT '{email}';
```

Default `'{email}'` — groups that never touch this field continue to work as today. Managers add `'gchat'` (or future channels) by connecting the integration; the backend appends to the array. Removing an integration removes the identifier.

`GET /api/v0/group-settings` already returns `notification_channels`; this column replaces the hard-wired `["email"]` slice in the handler package. The response key name stays `notification_channels`.

---

## System defaults

The system default is **individual email on, all broadcasts off**. Broadcast destinations (shared email, chat spaces) require a team to have a channel connected AND either the group default or team default to have that event enabled. This ensures groups that have never touched notification settings get sensible individual-email behaviour without any setup.

Individual email defaults are NOT controlled by the group defaults table. They cascade as: user explicit choice → team `individual_notifications_enabled` → system default (on).

---

## "Force defaults" — manager reset

Managers can push the current group/team defaults to all users, overwriting every user-level override in the group. Use case: the group has changed its setup significantly (e.g. added a shared team email and disabled individual notifications) and wants everyone to start from the new baseline.

```
POST /api/v0/group-settings/force-notification-defaults
```

Effect: sets `users.notification_prefs = '{}'` for every user in the group. In the updated UX, `{}` means "Följ avdelningsstandard" (middle column) — the next pref resolution for each user will pick up the current team and group defaults.

UI: A "Återställ alla användares notiser till standard" button in the group notification defaults section, behind a confirmation dialog. Returns a count of affected users.

---

## Phase 3.5a — Email Threading + Team Broadcast

### Goals

- Teams can define a shared `notification_email` address that receives one "broadcast" message per event (rather than N individual emails).
- Notification emails for the same booking/issue are threaded (Gmail/Outlook group them into one conversation).
- Three-tier preference resolution: Group defaults → Team defaults → User prefs.
- Personal user notification prefs use a three-column radio table (Alltid personlig e-post / Följ avdelningsstandard / Ingen personlig e-post) rather than a per-channel toggle table.
- Group defaults control **broadcast channel defaults only** (which event types fire the team email/GChat broadcast when a team has that channel connected, including manager-team events). Individual email defaults are not set at group level.
- `enabled_channels` drives which broadcast channel columns appear in the group defaults and team defaults UIs.

---

### Data model changes

#### `group_settings`

```sql
ALTER TABLE group_settings ADD COLUMN enabled_channels text[] NOT NULL DEFAULT '{email}';
```

#### `teams` — new columns

```sql
ALTER TABLE teams ADD COLUMN notification_email                text;
ALTER TABLE teams ADD COLUMN notification_prefs                jsonb    NOT NULL DEFAULT '{}';
ALTER TABLE teams ADD COLUMN individual_notifications_enabled  boolean  NOT NULL DEFAULT true;
```

| Column | Purpose |
|---|---|
| `notification_email` | Shared mailbox (e.g. `eagles@scoutgroup.org`). Null = no broadcast email for this team. |
| `notification_prefs` | Team-level defaults, same JSONB shape as `users.notification_prefs`. Keyed by `(event_type, channel)`. Missing keys fall back to group defaults. |
| `individual_notifications_enabled` | Default `true`. When `false`, personal channel delivery is *off by default* for team members. Any user can still override this explicitly in their own prefs — it is a team default, not a hard ceiling. |

Migration: `00008_team_notifications.sql`.

#### `notification_log` — threading columns

```sql
ALTER TABLE notification_log ADD COLUMN thread_key  text;   -- "booking_<uuid>" or "issue_<uuid>"
ALTER TABLE notification_log ADD COLUMN message_id  text;   -- email Message-ID stored after first send to this (thread_key, recipient)
```

`thread_key` is the lookup key across all channels. `message_id` is email-specific: stored after the first send so follow-ups can set `In-Reply-To`. For GChat (Phase 3.5b) the same `thread_key` value is passed as the Chat API `threadKey` parameter — no extra column needed.

---

### Dispatch logic

When an event fires:

```
1. Identify entity (booking or issue) and its associated team.

2. BROADCAST
   For each channel in group.enabled_channels:
   - email: if team.notification_email is set
            AND IsEnabled(team.notification_prefs → group_defaults → system, event, "email")
            → send one threaded email to notification_email
   (gchat: Phase 3.5b)

3. PERSONAL
   Collect recipients: booking creator + all team members (via users.team_ids, as today).
   For each recipient r:
   - Resolve personal_enabled = IsEnabled(r.prefs → team.prefs → group_defaults → system, event, channel, isManager)
   - If individual_notifications_enabled is false AND r has no explicit override → personal_enabled = false
   - If r's personal address == notification_email AND broadcast was sent → skip (already received broadcast)
   - If personal_enabled → send to r's address on that channel
```

The personal email uses `users.email` — which may differ from the shared mailbox. A user whose personal address matches the shared mailbox is never double-notified.

---

### Email threading

Every notification for the same entity uses a consistent thread key: `booking_<uuid>` or `issue_<uuid>`.

**First send** to a given `(thread_key, recipient)`:

1. Generate `Message-ID`: `<{thread_key}-{random}@{smtp_host}>`.
2. Send without `In-Reply-To`.
3. Insert `notification_log` row with `message_id` and `thread_key` set.

**Subsequent sends** to the same `(thread_key, recipient)`:

1. Look up `message_id` from `notification_log WHERE thread_key = ? AND recipient = ? LIMIT 1`.
2. Send with:
   ```
   In-Reply-To: <stored message_id>
   References:  <stored message_id>
   ```
3. Subject: `Re: [GroupName] Booking #1234 — Eagle Team` (same prefix, "Re: " prepended). Consistent prefix ensures Gmail/Outlook thread on subject even if headers are stripped.

**go-mail**: `Message-ID` is set via `msg.SetMessageIDWithValue(...)` before `DialAndSend`. Caller-generated; do not rely on the server's auto-generated ID.

**Implemented in**: `sendTo` in `send.go` handles all threading bookkeeping — looks up any prior `message_id` from `notification_log` for `(thread_key, user_id, channel)`, sets `msg.InReplyTo` if found, otherwise sets `msg.MessageID` for the first send, and writes the `notification_log` row (with `thread_key` + `message_id`) after send. The scheduler's overdue path has equivalent inline logic. `smtp.go` applies `In-Reply-To`/`References` headers from `msg.InReplyTo` and calls `SetMessageIDWithValue` from `msg.MessageID`. The broadcast path (step 3.5a-4) should call `q.LogNotification` directly and follow the same pattern, using the team email address as the pseudo-`user_id` key for thread lookup.

---

### Three-tier preference resolution

```go
func IsEnabled(ctx context.Context, q *db.Queries, userID, groupID, teamID, event, channel string, isManager bool) bool
```

Resolution order (first match wins):

**For broadcast channels (team email, gchat) — team/role events only:**
1. `teams.notification_prefs[event][channel]` — team explicit setting (resolved against the event's target team)
2. `group_settings.notification_defaults[event][channel]` — group broadcast default
3. `systemDefaults[event][channel]` — hardcoded fallback (broadcast off)

Broadcast only fires if the target team actually has the channel connected (`notification_email` set or `gchat_space_id` set). The group defaults table covers broadcast defaults for all team/role events, including manager events (where the "team" is the manager team).

**For individual email — personal events:**
Resolution is simple on/off from `users.notification_prefs[event]["email"]` → `systemDefaults[event]["email"]` (on). No team or group tier — `individual_notifications_enabled` does not apply.

**For individual email — team/role events:**
1. `users.notification_prefs[event]["email"]` — explicit user override (left/right column in personal tab)
2. If step 1 has no value AND `team.individual_notifications_enabled == false` → `false`
3. `systemDefaults[event]["email"]` — hardcoded fallback (on)

Group defaults do not participate in individual email resolution for any event category. The group defaults table is broadcast-only.

Pass `teamID = ""` for personal events to skip the team tier entirely.

`GET /api/v0/me/notification-prefs` — `source` field values: `"user"`, `"team_default"`, `"group_default"`, `"system"`. For team/role events this drives which radio column is pre-selected: explicit user value → left or right, no user pref → middle.

---

### Event categorisation

Notification events split into two categories with different UX treatment:

**Personal events** — the user is the named subject. Team broadcast is irrelevant; these always go to the individual.

| Event | Who receives | Notes |
|---|---|---|
| `issue_assigned_to_me` | The assignee | Non-toggleable — always on |
| `issue_resolved` | Reporter + all assignees | Simple on/off |
| `issue_commented` | Reporter + all assignees | Simple on/off |
| `booking_rejected` | Booking creator | Simple on/off |

**Team/role events** — the user is notified as a member of a team or role. A team broadcast (email or GChat) is a real alternative to individual delivery. Manager-targeted events go to the manager team and can have a shared broadcast destination like any other team.

| Event | Target team | Visible to |
|---|---|---|
| `booking_confirmed` | Booking's used-by team | All |
| `booking_cancelled` | Booking's used-by team | All |
| `booking_reminder` | Booking's used-by team | All |
| `booking_overdue` | Booking's used-by team | All |
| `booking_needs_approval` | Manager team | Managers only |
| `booking_submitted_no_approval` | Manager team | Managers only |
| `booking_any_created` | Manager team | Managers only |
| `issue_created` | Manager team | Managers only |

For team/role events, "which team's standard do I follow?" depends on the event: booking events resolve against the booking's used-by team; manager events resolve against the manager team. The middle-column behaviour follows the relevant team's `individual_notifications_enabled` flag.

**Which events are visible per team access level** (in the team settings tab and personal tab):
- `view` / `book` / `trusted`: booking events only (`booking_confirmed`, `booking_cancelled`, `booking_reminder`, `booking_overdue`). Trusted teams have no special notification distinction from book-level — the auto-approval difference is in the booking flow, not notifications.
- `manager`: all of the above plus manager events (`booking_needs_approval`, `booking_submitted_no_approval`, `booking_any_created`, `issue_created`).

Manager-event rows are never shown in the context of a non-manager team, even to users who are also managers in a separate team.

---

### Personal tab — notification preferences

The personal tab replaces the old flat toggle table with **two distinct sections**:

**Personliga notiser** — personal events. Simple on/off toggle per row. `issue_assigned_to_me` renders as a non-toggle informational row (always on, lock icon).

**Avdelnings- och rollnotiser** — team/role events. A **three-column radio table** (one row per event). Manager-only rows hidden for non-managers.

| Column | Swedish label | Meaning | Maps to in `users.notification_prefs` |
|---|---|---|---|
| Left | Alltid personlig e-post | Always send personal email, regardless of team broadcast | `{email: true}` |
| Middle | Följ avdelningsstandard | Inherit the relevant team's defaults (default state) | `{}` (no explicit override) |
| Right | Ingen personlig e-post | Never send personal email | `{email: false}` |

Middle column behaviour is controlled by the relevant team's `individual_notifications_enabled` flag. GChat has no personal delivery path (broadcast-only via spaces), so it does not appear as a column here.

A "Hantera avdelningsnotiser →" link near the Avdelnings- och rollnotiser heading leads to the team settings tab.

### Team tab — team notification settings

A new **"Avdelningar och roller"** tab on the profile page, positioned between the personal tab and the manager group tab. Visible to any user who belongs to at least one team.

**Team picker**: simplified chip/card grid — one card per team the user belongs to. Clicking selects the team and reveals a details panel.

**Details panel** (all team members can view and edit):

- **Namn** — editable text input with save button.
- **Notiser** section:
  - **Broadcast email input** (`notification_email`) — shared mailbox for team-wide email delivery.
  - **Standard för avdelningsmedlemmar** toggle (`individual_notifications_enabled`) — "Skicka inte personlig e-post till medlemmar som standard". Controls what happens for members on the "Följ avdelningsstandard" column in their personal tab.
  - **Per-event broadcast channel table** — rows = event types, columns = broadcast channels enabled by the group (team email, GChat space). Each cell is a checkbox (on/off). Greyed-out cells show the inherited group default when no explicit team value is set.
- **Integrationer** section (read-only for non-managers):
  - Shows the configured broadcast email address.
  - Shows the linked Google Chat space name, e.g. "Eagle Scouts (spaces/AAAA123)", or "Inget Google Chat-utrymme kopplat" if none.
  - Managers assign/unlink the GChat space from the Group tab.

### Group tab — GChat integration section (manager only, near SMTP settings)

**State A — not configured:**
- Short explanation of what the service account JSON is for.
- Textarea (full width, ~8 rows) to paste the raw service account JSON key.
- "Anslut Google Chat" button — calls `POST /group-settings/gchat-key`, shows inline error on failure (same style as SMTP errors).

**State B — configured:**
- Status line: connected indicator + admin email (`gchat_admin_email`).
- "Koppla bort" button — confirmation dialog before `DELETE /group-settings/gchat-key`.
- Expandable/collapsible "Teamkopplingar" section (collapsed by default):
  - Table: team name | Google Chat-utrymme dropdown.
  - Dropdown options: "Ingen" (top, clears mapping) + one option per available space formatted as "DisplayName (spaces/AAAA123)".
  - Selecting a space auto-saves via `PUT /teams/{id}/gchat-space`; "Ingen" calls `DELETE /teams/{id}/gchat-space`. No separate save button.

### i18n

All user-visible strings use Paraglide keys in `api/internal/i18n/messages/{sv,en}.json`. Swedish uses existing terminology: "avdelning" (troop), "roll" (role), "avdelningar och roller" (teams collectively). The word "lag" is not used.

---

### API changes (3.5a)

`GET /api/v0/teams/{id}` gains:

```json
{
  "notification_email": "eagles@scoutgroup.org",
  "individual_notifications_enabled": true,
  "notification_prefs": { ... }
}
```

`PUT /api/v0/teams/{id}` accepts the three new fields (general team update, name/type/access_level only).

Notification settings have dedicated endpoints to keep concerns separate:

```
GET  /api/v0/teams/{id}/notification-settings   (any team member)
PUT  /api/v0/teams/{id}/notification-settings   (any team member, partial update)
```

Authorization: the caller must belong to the team (their `team_ids` contains the team ID), or be a group manager. This is checked server-side, not just via role middleware.

`PUT` body: any combination of `notification_email`, `notification_prefs`, `individual_notifications_enabled`. Missing fields are left unchanged.

`GET /api/v0/group-settings` — `notification_channels` is now derived from `enabled_channels` column instead of a hard-wired slice.

```
POST /api/v0/group-settings/force-notification-defaults   (manager only)
```

Response: `{ "reset_count": 42 }`.

---

### Implementation steps

| Step | Deliverable | Status |
|---|---|---|
| 3.5a-1 | Migration `00008_team_notifications.sql` + sqlc queries | ✅ done |
| 3.5a-2 | Three-tier `IsEnabled` + `ResolvePrefs` (adds teamID param) | ✅ done |
| 3.5a-3 | Email threading: `Message-ID` generation + `In-Reply-To` headers in `smtp.go` | ✅ done |
| 3.5a-4 | Broadcast send to `team.notification_email` | ✅ done |
| 3.5a-5 | `enabled_channels` column + `notification_channels` response derived from DB | ✅ done |
| 3.5a-6 | `force-notification-defaults` endpoint | ✅ done |
| 3.5a-7 | Team notification settings UI (team detail page) | ✅ done |
| 3.5a-8 | Update `source` in prefs response / frontend label for `"team_default"` | ✅ done |
| 3.5a-9 | "Force defaults" button in group settings UI | ✅ done |

---

## Phase 3.5b — Google Chat Bot

### Prerequisites

- Google Workspace organisation for the scout group.
- Service account with Domain-Wide Delegation enabled in Google Admin Console (see [Google Chat setup guide](gchat-manager-guide.md)).
- Groups not on Workspace continue using email only. GChat is additive.

### Goals

- Managers upload a service account JSON key → backend validates and stores it encrypted.
- Manager maps scout teams to Google Chat Spaces via a UI in group settings.
- Bot is added to the space automatically on link; sends a "connected" welcome card.
- Booking/issue events fire a threaded card to the mapped space (broadcast only — no personal DMs).
- `"gchat"` is appended to `enabled_channels` on successful key upload; removed on key deletion.

### No personal DMs

GChat delivery is broadcast-only (Space messages). Personal DMs require knowing the user's Google identity and create significant complexity. Deferred to a future phase.

---

### Data model changes

#### `group_settings` — GChat auth

```sql
ALTER TABLE group_settings ADD COLUMN gchat_service_account_json_encrypted bytea;
ALTER TABLE group_settings ADD COLUMN gchat_admin_email text NOT NULL DEFAULT '';
```

Same `crypto.Encrypt`/`crypto.Decrypt` pattern as `smtp_key_encrypted`. Never returned in API responses; `gchat_configured: bool` is returned instead.

#### `teams` — GChat space

```sql
ALTER TABLE teams ADD COLUMN gchat_space_id text;   -- e.g. "spaces/AAAA123"
```

`gchat_webhook_url` is dropped in this migration (see note below).

Migration: `00009_gchat.sql`.

---

### GChat notifier

```go
type GChatNotifier struct {
    Q *db.Queries
}
```

Implements the same `Notifier` interface as `SMTPNotifier`. Uses the [Google Chat REST API v1](https://developers.google.com/workspace/chat/api/reference/rest). Auth: service account + DWD impersonating `gchat_admin_email`.

**Threading**: `POST spaces/{id}/messages` with `messageReplyOption=REPLY_MESSAGE_FALLBACK_TO_NEW_THREAD` and `thread.threadKey={thread_key}`. The Chat API creates or reuses the thread by key — no extra state needed beyond what 3.5a already stores.

**Card structure**:
- Header: group logo + event title + colour-coded banner (mirrors email template tone: orange/green/red/blue).
- Section: key fields (booking dates + team + status, or issue title + status + assignees).
- Footer: CTA button linking to the entity.

**Future channels (Slack, Teams)**: implement a `SlackNotifier` / `TeamsNotifier`, append the identifier to `enabled_channels` on connection, done. No changes to the dispatch loop, preference tables, or notification_log.

---

### Setup UI

Located in **Group Settings > Integrationer** (manager only).

**GChat section**:
1. "Anslut Google Chat" button → opens an upload dialog for the service account JSON key.
2. `POST /api/v0/group-settings/gchat-key` — backend validates by calling `spaces.list`. On success: appends `"gchat"` to `enabled_channels`, returns accessible spaces.
3. `gchat_configured: bool` + `gchat_admin_email` appear in `GET /api/v0/group-settings`.
4. "Koppla bort" button → `DELETE /api/v0/group-settings/gchat-key` — removes key, removes `"gchat"` from `enabled_channels`, clears all `teams.gchat_space_id` in the group.

**Team mapper** (visible once gchat_configured):
- Table: team name | Google Chat Space (dropdown) | status.
- Spaces fetched from `GET /api/v0/group-settings/gchat-spaces`. Spaces already linked to another team are excluded.
- On link: backend calls `spaces.members.create` to add the bot, then posts a welcome card to the space.
- On unlink: clears `gchat_space_id`; bot remains in the space (removal is manual).

---

### Dispatch logic extension

Phase 3.5a broadcast step is extended:

```
2. BROADCAST
   - email: (unchanged from 3.5a)
   - gchat: if team.gchat_space_id is set
            AND "gchat" in group.enabled_channels
            AND IsEnabled(team.notification_prefs → group_defaults → system, event, "gchat")
            → send threaded card to space
```

Personal step is unchanged — GChat has no personal delivery path in this phase.

---

### API changes (Phase 3.5b additions)

```
POST   /api/v0/group-settings/gchat-key        body: multipart JSON file  (manager only)
DELETE /api/v0/group-settings/gchat-key                                    (manager only)
GET    /api/v0/group-settings/gchat-spaces      → [{ "id", "name" }]      (manager only)
```

`GET /api/v0/group-settings` gains `gchat_configured: bool`. `notification_channels` gains `"gchat"` automatically once `enabled_channels` includes it.

---

### Implementation steps

| Step | Deliverable | Status |
|---|---|---|
| 3.5b-1 | Migration `00009_gchat.sql` (gchat columns; drop gchat_webhook_url) | ✅ done |
| 3.5b-2a | SQL queries updated (`group_settings.sql`, `teams.sql`) + `sqlc generate` run | ✅ done |
| 3.5b-2b | `GChatNotifier` (card builder, threading, DWD auth) | ✅ done |
| 3.5b-3 | gchat-key endpoints + enabled_channels update on connect/disconnect | ✅ done |
| 3.5b-4 | Team mapper UI + space link/unlink endpoints | 🚧 UI implemented, svelte-check pending |
| 3.5b-5 | Dispatch loop: gchat broadcast path | ✅ done |

**What was done in 3.5b-1/2a:**
- `00009_gchat.sql`: adds `gchat_service_account_json_encrypted bytea` and `gchat_admin_email text` to `group_settings`; adds `gchat_space_id text` to `teams`; drops `gchat_webhook_url` from `group_settings`, `teams`, and `users`.
- New queries in `group_settings.sql`: `SetGchatCredentials`, `ClearGchatCredentials`, `GetGchatCredentials`, `UpdateEnabledChannels`, `ClearAllGchatSpacesForGroup`.
- New queries in `teams.sql`: `SetTeamGchatSpace`, `ClearTeamGchatSpace`, `ListTeamsWithGchatInfo`.
- `sqlc generate` regenerated all files in `internal/db/` — do not edit those by hand.

**What was done in 3.5b-2b–3.5b-5:**
- `notifications/gchat.go`: `GChatNotifier` implements `Notifier`. Auth uses a service account JWT signed with `golang-jwt/jwt/v5` (already a dep — no new packages added) and exchanged for an OAuth2 bearer token. `Send()` posts a threaded text message to `spaces/{id}/messages` using `msg.ThreadKey` for Chat API thread keying. `ListGChatSpaces()` and `AddBotToSpace()` are exported helpers used by the handler layer.
- `notifications/notifier.go`: `Message` gains a `ThreadKey string` field (only consumed by `GChatNotifier`; `SMTPNotifier` ignores it).
- `notifications/send.go`: `sendBroadcastGChat()` mirrors `sendBroadcastEmail()` — checks team `gchat_space_id`, checks team prefs for the `"gchat"` channel, sends, and writes to `notification_log`. All five booking `Send*` functions gain a `gn Notifier` parameter and call both broadcast paths.
- `handler/group_settings.go`: removed stale `gchat_webhook_url` fields; added `gchat_configured bool` and `gchat_admin_email string` to the response; registered `POST /gchat-key`, `DELETE /gchat-key`, `GET /gchat-spaces`.
- `handler/teams.go`: registered `PUT /{id}/gchat-space` and `DELETE /{id}/gchat-space`.
- `handler/bookings.go`: `BookingHandler` gains `GChatNotifier notifications.Notifier`; all Send* calls pass it through.
- `cmd/server/main.go`: creates `&notifications.GChatNotifier{Q: queries}` and passes it (or `NoopNotifier{}` in demo mode) to `BookingHandler`.
- `web/src/lib/api/client.ts`: updated `GroupSettings` type; added `uploadGchatKey`, `deleteGchatKey`, `listGchatSpaces`, `setTeamGchatSpace`, `clearTeamGchatSpace` client methods.

**3.5b-4 UI — what was implemented:**

Backend additions (all compile-checked):
- New sqlc queries: `IsTeamMember`, `UpdateTeamName`, `GetTeamNotificationSettings` now returns `gchat_space_id`.
- `PUT /teams/{id}/notification-settings` and `GET /teams/{id}/notification-settings` opened to team members (membership checked via `IsTeamMember`; managers bypass the check). Previously manager-only.
- `PUT /teams/{id}/name` — new endpoint, team member accessible.
- `PUT /me/notification-prefs` now accepts `*bool` values; `null` removes an explicit user override, reverting that event+channel to team/group/system default (needed for the "Följ avdelningsstandard" middle radio column).
- `client.ts`: `TeamNotifSettings` gains `gchat_space_id`; `updateNotificationPrefs` accepts `boolean | null`; `updateTeamName` method added.
- i18n: ~30 new keys added to both `sv.json` and `en.json` (tab label, notification section headings, three-column radio labels, team settings labels, GChat section labels, force-defaults labels).

Frontend additions (`web/src/routes/profile/+page.svelte`):
- **New "Avdelningar och roller" tab** — between Profil and Gruppinställningar. Visible when user belongs to ≥1 team.
- **Team picker** — chip-style buttons, one per user team. Clicking loads team notification settings from the API.
- **Team detail panel** — name edit (all members), broadcast email input, suppress-individual toggle, per-event broadcast channel table (event set filtered by team access level: manager teams see manager events too), read-only integrations section (broadcast email + GChat space ID).
- **Personal tab notification section redesigned** — split into:
  - "Personliga notiser": simple on/off checkboxes for personal events (`booking_rejected`, `issue_assigned_to_me` locked, `issue_resolved`, `issue_commented`).
  - "Avdelnings- och rollnotiser": three-column radio table (`always` / `follow` / `never`) for team/role events. Manager-only rows hidden for non-managers.
- **GChat integration section** in the group tab (manager only), below SMTP. State A: textarea + connect button. State B: connected status + disconnect button + collapsible team-space mapper table (auto-saves on dropdown change).

**Still needed before this step is done:**
- Run `pnpm run build && pnpm run check` and fix any remaining svelte-check errors (10 errors were left at the end of the session — all related to missing i18n keys now added; need a rebuild to confirm zero errors).
- Smoke-test: visually verify the three new UI sections work in the browser.
- The group tab's existing team notification settings section (manager view) still uses `allBookingEvents`/`allIssueEvents` — confirm this renders correctly.

---

---

## Phase 3.6 — Per-event personal email policy + richer team settings UI

### Motivation

`individual_notifications_enabled` is a single boolean that applies to all events for a team. This is too coarse: teams that connect a broadcast email want personal emails to stop automatically, but the current model requires them to flip a toggle manually. At the same time, some events may still warrant personal delivery even when broadcast is set up (e.g. a booking rejection directed at the creator). Phase 3.6 replaces the boolean with a per-event **personal email policy** stored inside `notification_prefs`, giving teams the right default automatically and fine-grained control where needed.

### Gruppkanal — unified broadcast concept

Two roles govern which broadcast channels a team uses:

1. **Manager** — configures which integrations exist for a team (sets `notification_email`, links a GChat space). These are the *available* channels.
2. **Team members** — choose, at the team level, which of the available channels to include in their Gruppkanal. This is a **team-level selection**, not per event.

A team may have both email and GChat available (set up by the manager) but choose to only use GChat. Or choose both. Or neither. Once the Gruppkanal composition is decided, per-event it is just one Gruppkanal checkbox (on/off) — no per-event channel selection.

The Gruppkanal composition is stored per team (see Data model). Dispatch fires every channel the team has opted into for each event where Gruppkanal is on.

Group `enabled_channels` in `group_settings` controls which channel types the manager can configure. A channel type not in `enabled_channels` cannot appear in any team's Gruppkanal.

### Personal email policy

For each team/role event, a team (and the group defaults) stores one of three personal email policies:

| Policy value | Swedish label | Meaning |
|---|---|---|
| `always` | Alltid personlig | Send personal email to every team member regardless of Gruppkanal |
| `if_no_broadcast` | Personlig om gruppkanal saknas | Send personal email only if the team has no Gruppkanal configured; if Gruppkanal is available, use that instead |
| `never` | Aldrig personlig | Never send personal email to team members for this event |

**`if_no_broadcast` checks Gruppkanal composition, not delivery**: personal email is suppressed if the team's Gruppkanal includes at least one channel, regardless of whether delivery succeeds. A broken SMTP server would also fail personal delivery — no fallback is warranted.

**System default**: `if_no_broadcast`. A team that opts into any Gruppkanal channel automatically gets the right behaviour. Teams with an empty Gruppkanal continue to receive personal emails as before.

`individual_notifications_enabled` is **deprecated**. It remains in the schema until migrations are consolidated at release. The dispatch logic reads `personal_email_policy` from `notification_prefs` first; if absent it falls back to `individual_notifications_enabled`; if that is also absent it uses `if_no_broadcast`.

### Channel taxonomy

**Broadcast channels** (Gruppkanal) — one message to a shared destination per event, all fire together:
- `email` — team's `notification_email` address
- `gchat` — team's linked Google Chat Space
- Future: `slack`, `teams`, etc.

**Individual channels** — one message per recipient:
- `personal_email` — each member's own email address, governed by the personal email policy
- `push` (future) — web push to each member's device, see below

### Future: push notifications

Web push (PWA on smartphone) would add a new individual channel. It delivers to each member's device, not to a shared space — it is architecturally individual, not broadcast. It would:
- Have its own per-event policy: `push_policy` with the same three options (`always` / `if_no_broadcast` / `never`), stored in `notification_prefs`
- Appear as a separate row in the personal tab (not a Gruppkanal column), since push is personal
- The group can enable/disable the push channel via `enabled_channels`

No implementation yet — documented here to keep the data model extensible.

---

### UI specification (UI first)

> **Implementation order**: build and validate the UI against hardcoded/mocked data before touching the backend. This makes UX problems visible early.

#### Team tab — revised detail panel

**Gruppkanal section** (above the per-event table): shows the available channels the manager has set up for this team, with a checkbox for each. The team opts into whichever they want included in Gruppkanal. This selection applies to all events — not per event.

Example (team has both email and GChat available):
```
Gruppkanal
☑ Grupp-e-post  (scouts@example.org)
☐ Google Chat   (Eagle Scouts)
```

If the manager has set a group default for Gruppkanal composition, teams that have not made their own selection show the default with a "(standard)" badge and can override it.

If no channels are available (manager has not set up any integrations for this team), the Gruppkanal section shows a note: "Inga grupputskick kopplade — kontakta en ansvarig."

**Per-event table** (below Gruppkanal section): rows = event types visible for this team. Columns:

| Column | When shown | Control type |
|---|---|---|
| **Personlig e-post** | Always | Compact 3-option radio |
| **Gruppkanal** | Team's Gruppkanal is non-empty | Checkbox |
| **Push** (future) | Group has `push` in `enabled_channels` | Checkbox |

The 3-option radio for **Personlig e-post** uses short labels:
- Alltid
- Om ej gruppkanal *(dimmed when team Gruppkanal is empty — equivalent to "Alltid"; show tooltip)*
- Aldrig

The Gruppkanal checkbox column is omitted if the team's Gruppkanal is empty (nothing opted into). When shown, checking a row fires all opted-in channels for that event.

Cells showing the group default (no explicit team override) render lighter with a "(standard)" badge.

The **"Skicka inte personlig e-post till medlemmar som standard"** toggle is removed.

**Auto-expand first team**: when the user navigates to the Avdelningar och roller tab, the first team in the list is automatically selected and its detail panel opened.

**Mobile layout**: Gruppkanal section stacks vertically. Per-event table collapses to cards — event label, then the radio and Gruppkanal checkbox on the same row.

#### Group defaults tab — revised

Two parts mirroring the team tab structure:

**Default Gruppkanal composition** (above the per-event table): checkboxes for each channel type available in the group (`enabled_channels`). Sets the default opted-in channels for teams that have not chosen their own composition. Teams can override.

Example:
```
Standardval för gruppkanal
☑ Grupp-e-post
☑ Google Chat
```

**Per-event table**: same two columns as team settings (Personlig e-post radio + Gruppkanal checkbox). Sets the per-event defaults for teams without explicit settings.

- **Personlig e-post** column: system default = `if_no_broadcast`, shown with "(systemstandard)" badge.
- **Gruppkanal** column: system default = on for most events (`BroadcastSystemDefaults`). Always shown (applies to any team that has a non-empty Gruppkanal).

#### Personal tab — unchanged

The three-column radio (Alltid / Följ avdelningsstandard / Aldrig) is unchanged. "Följ avdelningsstandard" follows the team's `personal_email_policy` for each event.

---

### Data model changes

#### `teams` — new column

```sql
ALTER TABLE teams ADD COLUMN gruppkanal_channels text[];   -- NULL = inherit group default
```

Three meaningful states:

| Value | Meaning |
|---|---|
| `NULL` | No explicit selection — inherits `default_gruppkanal_channels` from group settings |
| `'{}'` (empty array) | Team has explicitly opted out of all broadcast channels |
| `'{email,gchat}'` | Team has explicitly chosen these channels |

The manager sets up which channels are *available* for a team (via `notification_email` / `gchat_space_id`); the team picks from those available channels. A channel can only appear in `gruppkanal_channels` if it is actually configured — the backend enforces this on every write and automatically removes a channel when its integration is unlinked.

#### `group_settings` — new column

```sql
ALTER TABLE group_settings ADD COLUMN default_gruppkanal_channels text[] NOT NULL DEFAULT '{}';
```

Group-level default Gruppkanal composition. Teams with `gruppkanal_channels IS NULL` inherit this. When the group admin adds a new broadcast channel to the group and sets it in `default_gruppkanal_channels`, all `NULL`-teams pick it up automatically.

**Force-to-default**: the existing `POST /api/v0/group-settings/force-notification-defaults` endpoint is extended to also reset all teams' `gruppkanal_channels` to `NULL`, so they inherit the current `default_gruppkanal_channels`. The endpoint already resets user prefs; this adds team Gruppkanal to the same operation. The UI button label and confirmation dialog should reflect both effects.

#### `notification_prefs` JSONB — revised shape

The per-event JSONB shape is simplified. The old per-channel `email`/`gchat` booleans are replaced by a single `gruppkanal` boolean (on/off for all opted-in channels) and `personal_email_policy`:

```json
{
  "booking_confirmed": {
    "gruppkanal": true,
    "personal_email_policy": "if_no_broadcast"
  },
  "booking_needs_approval": {
    "gruppkanal": true,
    "personal_email_policy": "always"
  }
}
```

Which channels actually fire is determined by `gruppkanal_channels` (team column or group default) — not by per-event channel flags. Missing `gruppkanal` key → fall back to group defaults → system default (on). Missing `personal_email_policy` → fall back to group defaults → system default (`if_no_broadcast`).

This shape applies to `teams.notification_prefs`, `group_settings.notification_defaults`, and `users.notification_prefs` (users only store `personal_email_policy` — they have no Gruppkanal key).

Migration: `00010_gruppkanal.sql` (new file, alongside existing migrations; consolidated at release).

### Dispatch logic changes

**Resolve effective Gruppkanal channels** (helper, used in both broadcast and personal steps):

```
effectiveChannels(team) =
  if team.gruppkanal_channels IS NOT NULL → team.gruppkanal_channels
  else → group_settings.default_gruppkanal_channels
```

Only channels that are actually configured for the team appear in `gruppkanal_channels` — the backend enforces this — so no availability check is needed at dispatch time.

**Broadcast step**:

```
channels = effectiveChannels(team)
for channel in channels:
  if not IsEnabled(team.prefs → group_defaults → system, event, "gruppkanal") → skip
  → fire channel
```

`IsEnabled` now looks up the `gruppkanal` key in `notification_prefs` (not per-channel keys).

**Personal email step** for team/role events (replaces `individual_notifications_enabled` check):

```
1. Read policy = team.notification_prefs[event]["personal_email_policy"]
                 ?? group_defaults[event]["personal_email_policy"]
                 ?? "if_no_broadcast"
2. if policy == "never"  → skip personal email for all members
3. if policy == "always" → send personal email to each member (subject to user's own radio)
4. if policy == "if_no_broadcast":
     if effectiveChannels(team) is non-empty → skip personal email
     else → send personal email to each member
5. User's own explicit radio always overrides the team policy:
     "Alltid" → send regardless of policy
     "Aldrig" → skip regardless of policy
     "Följ standard" → apply policy from steps 2–4
```

Step 4 checks `effectiveChannels` (the team's opted-in set). A team that has channels available but whose effective Gruppkanal is empty (e.g. group default is also `'{}'`) still receives personal email.

### API changes

- `GET /teams/{id}/notification-settings` — returns `gruppkanal_channels` (null or array), `available_channels` (channels the manager has configured for this team — derived from `notification_email`/`gchat_space_id`), and `notification_prefs` using the new JSONB shape.
- `PUT /teams/{id}/notification-settings` — accepts `gruppkanal_channels` (null to reset to inherit, array to set explicit). Backend validates that every channel in the array is in `available_channels`; rejects otherwise.
- `GET /api/v0/group-settings/notification-defaults` — gains `default_gruppkanal_channels` and returns `notification_defaults` in the new JSONB shape (`gruppkanal` + `personal_email_policy` per event).
- `PUT /api/v0/group-settings/notification-defaults` — accepts `default_gruppkanal_channels` alongside the per-event JSONB.
- `POST /api/v0/group-settings/force-notification-defaults` — extended: now also sets all teams' `gruppkanal_channels` to `NULL`. Response: `{ "reset_user_count": 42, "reset_team_count": 7 }`.

When a manager unlinks a channel from a team (clears `notification_email` or `gchat_space_id`), the backend automatically removes that channel from `gruppkanal_channels` if present.

No new endpoints.

### Implementation steps

| Step | Deliverable | Status |
|---|---|---|
| 3.6-1 | Auto-expand first team on Avdelningar och roller tab open | ✅ done |
| 3.6-2 | Migration `00010_gruppkanal.sql`: `teams.gruppkanal_channels text[]` (nullable); `group_settings.default_gruppkanal_channels text[] NOT NULL DEFAULT '{}'`; drop `teams.individual_notifications_enabled` | ✅ done |
| 3.6-3 | sqlc queries: `GetTeamNotificationSettings` returns new columns; `SetTeamGruppkanalChannels`; `GetGroupNotificationDefaults`/`SetGroupNotificationDefaults` updated | ✅ done |
| 3.6-4 | Backend — channel availability enforcement: auto-remove channel from `gruppkanal_channels` when integration is unlinked; validate on PUT | ✅ done |
| 3.6-5 | Backend — extend `force-notification-defaults` to also NULL-out all teams' `gruppkanal_channels` | ✅ done |
| 3.6-6 | Backend — dispatch: `EffectiveGruppkanalChannels()` helper; `IsGruppkanalEnabled`; `ResolvePersonalEmailPolicy` in `send.go` and `scheduler.go` | ✅ done |
| 3.6-7 | Backend — `personal_email_policy` and `gruppkanal` in `BroadcastSystemDefaults`; group defaults and team settings API responses updated | ✅ done |
| 3.6-8 | i18n keys for new labels (Gruppkanal, radio options, column headers, force-defaults updated confirmation) | ✅ done |
| 3.6-9 | Team settings UI: Gruppkanal composition selector + unified per-event table (Personlig e-post radio + Gruppkanal checkbox); remove `individual_notifications_enabled` toggle | ✅ done |
| 3.6-10 | Group defaults UI: default Gruppkanal composition checkboxes + same per-event table; update force-defaults confirmation dialog | ✅ done |

### Outstanding fixes (3.6 follow-up)

~~**Team settings per-event table — simplify personal email control**~~ ✅ done

Implemented as context-aware binary toggle:
- **No Gruppkanal active**: single "Personlig e-post" checkbox. Unchecked → `personal_email_policy: "never"`. Checked/unset → omit key (default `always`).
- **Gruppkanal active**: "Gruppkanal" checkbox (existing) + "Skicka också personlig e-post" checkbox. Checked → `personal_email_policy: "always"`. Unchecked/unset → omit key (default `if_no_broadcast` suppresses personal).

Group defaults UI keeps three-way radio. Middle option relabelled "Föredra gruppkanal" via new key `page_profile_group_defaults_notif_personal_prefer_broadcast`. Removed `page_profile_teams_notif_personal_if_no_broadcast` and `_dimmed_tooltip`; added `page_profile_teams_notif_reset`. `page_profile_teams_notif_personal_always` and `_never` still used by group defaults radio.

~~**Dev-seed: set sane defaults for groups with broadcast email**~~ ✅ done (`default_gruppkanal_channels: ["email"]` already present)

~~**`00010_gruppkanal.sql` cleanup**~~ ✅ done — migration is schema-only, no seed data.

---

## Remaining work

- **3.5b-4 smoke-test**: visually verify team tab, GChat integration section, and group defaults table in the browser.
- **Step 10 email templates**: visual review via Mailpit (`http://localhost:8025`) + extend `TestNotifications_EventTriggered` to assert body contains booking URL, dates, item list.

Deferred items (personal email override, GChat richness, Slack/Teams, push notifications, logo in web header) moved to `docs/BACKLOG.md`.
