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

The system default is **individual email only**. Broadcast destinations (shared email, chat spaces) are always off by default and require explicit manager configuration. This ensures groups that have never touched notification settings get sensible behaviour without any setup.

---

## "Force defaults" — manager reset

Managers can push the current group/team defaults to all users, overwriting every user-level override in the group. Use case: the group has changed its setup significantly (e.g. added a shared team email and disabled individual notifications) and wants everyone to start from the new baseline.

```
POST /api/v0/group-settings/force-notification-defaults
```

Effect: sets `users.notification_prefs = '{}'` for every user in the group (same as each user pressing "Återställ till standard"). The next pref resolution for each user will pick up the current group and team defaults.

UI: A "Återställ alla användares notiser till standard" button in the group notification defaults section, behind a confirmation dialog. Returns a count of affected users.

---

## Phase 3.5a — Email Threading + Team Broadcast

### Goals

- Teams can define a shared `notification_email` address that receives one "broadcast" message per event (rather than N individual emails).
- Notification emails for the same booking/issue are threaded (Gmail/Outlook group them into one conversation).
- Three-tier preference resolution: Group defaults → Team defaults → User prefs.
- Personal user notification prefs remain unchanged in the UI (flat per-group table on `/profile`). No per-team context switcher.
- `enabled_channels` drives which columns appear in all preference UIs.

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

1. `users.notification_prefs[event][channel]` — explicit user override
2. If team exists AND `individual_notifications_enabled == false` AND step 1 had no explicit value → `false`
3. `teams.notification_prefs[event][channel]` — team default
4. `group_settings.notification_defaults[event][channel]` — group default
5. `systemDefaults[event][isManager]` — hardcoded fallback

Pass `teamID = ""` for events with no team context (some issue events) to skip tiers 2–3.

`GET /api/v0/me/notification-prefs` — `source` field gains a new possible value: `"team_default"`. Frontend label: "(teamstandard)".

---

### Team notification settings UI

Located on the **Team Detail page** (`/teams/[id]`, manager/trusted only), new "Notiser" section:

- **Broadcast email**: text input for `notification_email`. Shown for all groups (email is always an enabled channel).
- **Suppress individual notifications toggle**: "Skicka inte notiser till enskilda medlemmar som standard". Hint: "Medlemmar kan fortfarande aktivera egna notiser i sin profil."
- **Team notification defaults table**: same toggle-table layout as the group defaults table. Columns driven by `group.enabled_channels` — only active channels appear.

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
GET  /api/v0/teams/{id}/notification-settings   (manager only)
PUT  /api/v0/teams/{id}/notification-settings   (manager only, partial update)
```

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

| Step | Deliverable |
|---|---|
| 3.5b-1 | Migration `00009_gchat.sql` (gchat columns; drop gchat_webhook_url) |
| 3.5b-2 | `GChatNotifier` (card builder, threading, DWD auth) |
| 3.5b-3 | gchat-key endpoints + enabled_channels update on connect/disconnect |
| 3.5b-4 | Team mapper UI + space link/unlink endpoints |
| 3.5b-5 | Dispatch loop: gchat broadcast path |

---

## Open questions / backlog

- **`gchat_webhook_url` data**: check for any existing rows before dropping the column in 3.5b-1.
- **Slack / Teams**: when adding, follow the same pattern — new `Notifier`, append channel id to `enabled_channels`, implement a setup UI section in Integrationer. No other changes needed.
- **Personal GChat DMs**: deferred. Would require storing the user's Google identity at login (only feasible if OIDC provider is Google Workspace).
- **Step 6 TODO from Phase 3**: "when changing back to the default setting, indicate we are on the default setting" — addressable in 3.5a-8 when `source` gains `"team_default"`.
