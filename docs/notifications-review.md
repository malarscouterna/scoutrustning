# Notifications feature — review brief

Branch: `feat/notifications` → `main`
Commits: 25 (see `git log main...HEAD --oneline`)
Diff: 93 files, ~16 000 insertions, ~615 deletions

---

## What this feature does

Adds a full three-channel notification system to the booking/issue workflow:

- **Email** — per-event SMTP emails with threading (`In-Reply-To`), HTML templates, and a shared team broadcast address.
- **Google Chat** — service-account bot posts two-message threaded cards to team Spaces (broadcast only, no personal DMs).
- **Scheduled** — daily reminder (day before) and overdue alert, run by a background scheduler.

Preferences are three-tier: group defaults → team defaults → user override. Each event can be sent to a shared "Gruppkanal" (team broadcast) and/or individual members, with a per-event personal-email policy (`always` / `if_no_broadcast` / `never`).

---

## Line count breakdown

The headline number (16k insertions) is inflated by several categories that need little review:

| Category | Lines | Notes |
|---|---|---|
| `pnpm-lock.yaml` | 2 281 | Auto-generated lockfile — skip |
| sqlc-generated (`*.sql.go`, `models.go`) | ~700 | Auto-generated from SQL — review the `.sql` source files instead |
| i18n JSON (`sv.json` + `en.json`) | ~410 | Key-value pairs, verify completeness not logic |
| Email/GChat templates (`templates/`, `*.mjml`) | ~1 700 | Markup, verify rendering not logic |
| Design docs (`docs/`) | ~1 850 | Context only |
| `dev-seed.sh` | 116 | Dev tooling |

**Real application code to review: ~9 000 lines across ~30 files.**

---

## Files worth close attention

### Backend — new systems

| File | Lines | What to look at |
|---|---|---|
| `api/internal/notifications/prefs.go` | 283 | Three-tier `IsEnabled` resolution, `BroadcastSystemDefaults`, `PersonalEventKeys` vs `TeamEventKeys` split |
| `api/internal/notifications/send.go` | 606 | Per-event dispatch: broadcast email, broadcast GChat, personal email. Check `managerTeamID()` usage for manager-targeted events, double-notification guard (personal addr == broadcast addr) |
| `api/internal/notifications/scheduler.go` | 206 | Reminder and overdue scheduling; check deduplication via `notification_log` |
| `api/internal/notifications/gchat.go` | 377 | Service account JWT flow, space listing, `AddBotToSpace`, two-message thread pattern |
| `api/internal/notifications/smtp.go` | 119 | `In-Reply-To`/`References` threading headers, `Message-ID` generation |
| `api/internal/notifications/template.go` | 761 | Template data structs and rendering helpers. Verbose but no complex logic — skim for correctness of field mapping |

### Backend — handler changes

| File | Lines | What to look at |
|---|---|---|
| `api/internal/handler/group_settings.go` | 279 | GChat key endpoints (`POST/DELETE /gchat-key`, `GET /gchat-spaces`), demo-mode guards, `enabled_channels` management, `ListSpacesFn` injection point |
| `api/internal/handler/teams.go` | 267 | `SetGChatSpace`/`ClearGChatSpace`, `requireTeamMembership` (members can edit their own team's settings), Gruppkanal channel validation on write |
| `api/internal/handler/notification_prefs.go` | 188 | `GET/PUT /me/notification-prefs`, `GET/PUT /teams/{id}/notification-settings`, `GET/PUT /group-settings/notification-defaults`, `POST /force-notification-defaults` |
| `api/internal/handler/me.go` | 244 | `POST /me/test-email`, persona skip logic |
| `api/cmd/server/main.go` | 147 | Wiring — verify `GChatNotifier` vs `NoopNotifier` in demo mode |

### Frontend

| File | Lines | What to look at |
|---|---|---|
| `web/src/routes/profile/+page.svelte` | 1 281 | Three-tab profile page: personal prefs, team settings (Avdelningar och roller), group settings (GChat config + team mapper). Large single file — consider whether it needs splitting post-merge |
| `web/src/routes/issues/[id]/+page.svelte` | 134 | Assignee picker UI |
| `web/src/lib/api/client.ts` | 96 | New API client methods for all notification endpoints |

### Migrations (review the SQL, not the generated Go)

| File | What changed |
|---|---|
| `00008_team_notifications.sql` | Adds `notification_email`, `notification_prefs`, `individual_notifications_enabled` to `teams` |
| `00009_gchat.sql` | Adds GChat columns to `group_settings` and `teams`; drops stale webhook columns |
| `00010_gruppkanal.sql` | Adds `gruppkanal_channels` to `teams`, `default_gruppkanal_channels` to `group_settings`, drops `individual_notifications_enabled` |
| `00011_user_notification_email.sql` | Adds `notification_email` to `users` for personal email override |

---

## Security surface

- **GChat service account JSON** — stored as `AES-256-GCM` encrypted blob (`gchat_service_account_json_encrypted`). Decrypted bytes exist only in memory during token exchange. Key is `SETTINGS_ENCRYPTION_KEY` env var. Never logged, never returned in API responses (`gchat_configured: bool` only).
- **SMTP password** — same encryption pattern (`smtp_key_encrypted`). Only `smtp_key_masked` (first 3 + last 4 chars) returned in responses.
- **Demo mode** — `POST/DELETE /gchat-key` and `PUT /group-settings` (SMTP fields) return `403`. Event handlers and scheduler receive `NoopNotifier`. Worth verifying the `DemoMode` flag reaches every relevant handler.
- **Multi-tenancy** — all notification queries filter on `group_id`. Worth spot-checking `GetGchatCredentials`, `ListTeamsWithGchatInfo`, and `GetManagerTeam`.
- **Team settings access** — `GET/PUT /teams/{id}/notification-settings` and `PUT/DELETE /teams/{id}/gchat-space` are open to team members (not manager-only), verified via `requireTeamMembership`. Intentional — teams self-manage their broadcast settings.

---

## Known gaps (not blocking merge, tracked in backlog)

- No personal GChat DMs — broadcast to Spaces only.
- `notification_log` has no retention policy. Rows accumulate indefinitely.
- No alerting on `status = 'failed'` rows in `notification_log`.
- `SETTINGS_ENCRYPTION_KEY` has no rotation story — changing it post-deploy breaks all stored credentials.
- `profile/+page.svelte` is large enough that it may benefit from component extraction in a follow-up.

---

## Remaining work (post-refactor)

### Minor — `Update` handler in `group_settings.go` makes two separate DB writes without a transaction

`UpsertGroupSettings` followed by `UpdateSmtpSettings`. If the second write fails, non-SMTP fields are already persisted. Requires either combining into one SQL query or wrapping in a `pgx.Tx`. The handler currently only holds `*db.Queries`; passing the pool through would enable transactions.

### Minor — SMTP key is decrypted on every `GET /group-settings`

`settingsToResponse` (`group_settings.go:434–437`) decrypts the full key on every admin GET solely to produce the masked display value. Store the masked value in the DB alongside the encrypted blob (computed once on write) to avoid a crypto round-trip on every read.

### Minor — profile page `+page.svelte` should be split into components

`web/src/routes/profile/+page.svelte` (1281 lines) covers three tabs: personal prefs, team settings, and group/GChat config. Each tab is large enough to be its own component. Suggested split:
- `ProfileNotificationPrefs.svelte` — personal per-event preference table
- `ProfileTeamSettings.svelte` — Avdelningar och roller tab
- `ProfileGroupSettings.svelte` — GChat config + team space mapper

---

## Review findings (resolved)

### Duplication — `SendBookingConfirmed` and `SendBookingCancelled` are near-identical

Extracted `sendBookingToTeam(ctx, q, n, gn, b, event, baseURL)`. Both public functions are now one-liners.

### Duplication — `issueBroadcastTexts` and `issueBroadcastEmail` called `fetchIssueEmailData` twice

Replaced both helpers with `issueBroadcastData` which fetches once and returns `(msg Message, opener, detail string)`. Updated `SendIssueCreated` and `sendIssueBroadcastAndPersonal`.

### Duplication — three `from*Row` functions shared an identical language-defaulting pattern

Extracted `langOrDefault(v pgtype.Text) string`. Each `from*Row` function is now a single-line struct literal.

### Minor — `bookingThreadKey` and `issueThreadKey` called `teamIDStr` on entity IDs

Renamed `teamIDStr` → `formatUUID` across `prefs.go`, `send.go`, and `scheduler.go`.

---

### Bug — `PolicyIfNoBroadcast` suppression is inconsistent between `sendTo` and `sendOverdueForBooking`

`sendTo` (`send.go:67–87`) checks whether a channel is both opted-in *and* actually configured (has a real endpoint):
```go
if ch == "email" && team.HasNotificationEmail { hasConfiguredBroadcast = true }
if ch == "gchat"  && team.HasGchatSpace        { hasConfiguredBroadcast = true }
```
`sendOverdueForBooking` (`scheduler.go:137–144`) only checks `len(effective) > 0`. A team with `["email"]` in their channels but no `notification_email` address will have the overdue personal email suppressed even though no broadcast actually fires. Fix: extract the configured-broadcast check from `sendTo` into a shared helper and use it in both places.

---

### Duplication — `SendIssueResolved` and `SendIssueCommented` are near-identical

`send.go:551–577` and `send.go:579–605` share the same structure: fetch reporter, fetch assignees, build recipients, send broadcast + personal. The only difference is the event constant. Extract a private helper:
```go
func sendIssueBroadcastAndPersonal(ctx, q, n, gn, issue, event, baseURL string)
```
~50 lines eliminated.

### Duplication — `SendBookingNeedsApproval` and `SendBookingSubmittedNoApproval` are identical

`send.go:426–456`. Same structure, different event constant. Same extraction applies.

### Duplication — inline IIFE for creator+members recipient list

`SendBookingConfirmed` and `SendBookingCancelled` both contain:
```go
dedup(append(func() []recipient { if r, ok := bookingCreator(...); ok { return []recipient{r} } return nil }(), teamMembers(...)...))
```
The `bookingRecipients` helper already exists in `scheduler.go:199–206` and does exactly this. Move it to `send.go` (it belongs there) and use it in both places.

### Duplication — `sendOverdueForBooking` reimplements email threading from `sendTo`

`scheduler.go:155–193` manually builds `MessageID`, calls `GetThreadMessageID`, calls `LogNotification` — the exact same block that lives inside `sendTo`. The reason is that the scheduler resolves policy first and calls `n.Send` directly, bypassing `sendTo`. Either thread the policy check into `sendTo` (pass policy explicitly), or extract the threading+logging block into a helper both can call.

---

### Design — `sendBroadcastGChat` type-asserts to `*GChatNotifier`

`send.go:239` and `send.go:263`:
```go
if n, ok := gn.(*GChatNotifier); ok && n.LabelTeam { ... }
if gcn, ok := gn.(*GChatNotifier); ok { gcn.SendPaired(...) }
```
This breaks the `Notifier` abstraction. The `SendPaired` method is on the concrete type, making any alternative notifier (mock, future implementation) silently fall back to the slower two-`Send` path. Introduce a second interface:
```go
type PairedNotifier interface {
    Notifier
    SendPaired(ctx, groupID, space, opener, detail, threadKey string) (error, error)
}
```
and type-assert to `PairedNotifier`. The `LabelTeam` field access can be removed if the label is applied at the `GChatNotifier.SendPaired` level instead.

---

### Performance — N+1 DB queries per event dispatch

A single `SendBookingNeedsApproval` with 10 manager recipients hits:
- `sendBroadcastEmail` → `GetTeamNotificationSettings` (1) + `GetTeamNotifSettings` (2) + `GetGroupNotificationDefaults` (3) + `IsGruppkanalEnabled` → `GetTeamNotifSettings` (4) + `GetGroupNotificationDefaults` (5)
- `sendBroadcastGChat` → same pattern again (6–10)
- Per recipient (×10): `ResolvePersonalEmailPolicy` → `GetUserNotificationPrefs` + `GetTeamNotifSettings` + `GetGroupNotificationDefaults` (30 more)

~40 queries for a 10-recipient event. The team settings and group defaults do not change mid-dispatch. Fix: resolve `teamSettings` and `groupDefaults` once per event and pass them as parameters into `sendBroadcastEmail`, `sendBroadcastGChat`, and `sendTo`.

### Performance — `sendBroadcastEmail` fetches team settings twice

`send.go:310–317`: calls `GetTeamNotificationSettings` (to check `NotificationEmail`), then immediately calls `GetTeamNotifSettings` (which calls `GetTeamNotificationSettings` again). Merge: the second call already returns `HasNotificationEmail`, so the first query can be dropped.

---

### Correctness — `bookingThreadKey` passes booking ID to `teamIDStr`

`send.go:415–418`:
```go
func bookingThreadKey(b db.Booking) string {
    return "booking_" + teamIDStr(b.ID)  // b.ID is the booking UUID, not a team ID
}
```
The function name `teamIDStr` is misleading — it formats any `pgtype.UUID`. Consider renaming to `uuidStr` or `formatUUID`, or just inlining `b.ID.String()`. This is cosmetic but causes confusion during audits.

---

### Correctness — `GetGroupDefaults` handler builds merged view from system defaults only

`notification_prefs.go:115–128` iterates over `sys` (system defaults) and overlays group overrides. If a group admin ever writes a custom event key (e.g. a future event not in `BroadcastSystemDefaults`), it would be silently dropped from the response. Fine for now, but worth noting if the event list grows.

---

### Validation — user-supplied `PersonalEmailPolicy` is not validated on write

`notification_prefs.go:PutMe` stores any string in `personal_email_policy` without checking it's one of `always`, `if_no_broadcast`, `never`. Invalid values fall back to `PolicyAlways` in resolution (the final `return PolicyAlways` in `ResolvePersonalEmailPolicy`), so no security impact, but stale junk accumulates in the DB and makes future reads confusing. Add a validity check before writing.

### Validation — `PutGroupDefaults` accepts arbitrary event keys

`notification_prefs.go:PutGroupDefaults` stores whatever keys the manager submits. Add a validation loop against `notifications.AllEvents`.

---

### Atomicity — `Update` handler in `group_settings.go` makes two separate DB writes

`group_settings.go:214–252`: `UpsertGroupSettings` followed by `UpdateSmtpSettings`. If the second write fails, the group settings are partially updated (non-SMTP fields saved, SMTP fields not). Wrap in a transaction or collapse into a single query.

---

### Minor — SMTP key is decrypted on every `GET /group-settings`

`group_settings.go:434–437`: the key is decrypted on every read solely to produce the masked display value. Store the masked value alongside the encrypted blob (or compute it once on write) to avoid a crypto operation on every settings GET.

### Minor — GChat space listing doesn't handle pagination

`gchat.go:184`: `pageSize=100` but no `nextPageToken` handling. An org with >100 spaces gets silently truncated. Add a pagination loop.

### Minor — one GChat notification = two HTTP round-trips (token exchange + message post)

`gchatBotToken` is called on every `Send`/`SendPaired` with no caching. For bursty events (many teams, many bookings) this can slow things down. A short-lived in-memory cache keyed on `(groupID, scope)` with TTL < 1 hour would eliminate redundant token exchanges.

---

## Test coverage

- `notifications_test.go` — event-triggered sends (booking + issue), test-email endpoint, scheduled reminders and overdue
- `notification_dispatch_test.go` — three-tier preference resolution, Gruppkanal channel routing
- `notification_prefs_test.go` — GET/PUT prefs endpoints, force-defaults
- `scheduled_notifications_test.go` — reminder/overdue scheduling, deduplication
- `gchat_test.go` — key upload/delete, list spaces, team space link/unlink (uses injected stubs, no real GChat calls)
- End-to-end smoke test with a real GChat space was done manually in dev before merge.
