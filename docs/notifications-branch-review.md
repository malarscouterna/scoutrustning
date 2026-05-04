# feat/notifications — Pre-merge Review

Review of the full `feat/notifications` branch against `main`, conducted 2026-05-03.

---

## Critical — blocks merge

### Personal notification prefs are broken end-to-end

The GET/PUT contract changed during development but the frontend and tests were never updated.

**GET shape mismatch**

`/me/notification-prefs` returns `ResolvedPrefs = map[EventKey]ResolvedPref` where each value has `policy`, `source`, and `default_policy` fields. The TypeScript type `NotificationPrefs` in `web/src/lib/api/client.ts` still says `Record<string, Record<string, ChannelPref>>` — a two-level map keyed by channel name with `enabled/source/default_enabled` fields. The profile page reads `notifPrefs[key]['email'].enabled` and `.source`, which are undefined on the actual response.

**PUT shape mismatch**

`PutMe` in `api/internal/handler/me.go` reads and stores `map[string]map[string]bool` (e.g. `{"booking_confirmed":{"email":true}}`). `ParseNotificationPrefs` deserializes into `PerEventPrefs{Gruppkanal *bool, PersonalEmailPolicy string}`, so those keys silently produce a zero-value struct and every user preference write is discarded.

**Tests confirm the breakage**

`api/internal/handler/tests/notification_prefs_test.go` fails on all sub-tests — the helpers still expect the old three-level shape `prefs[event][channel][field]`.

**What needs to happen**

- Update `PutMe` to accept `{personal_email_policy: "always"|"never"|...}` per event (matching the shape the team-level settings already use), or add an explicit translation layer.
- Update the TypeScript `NotificationPrefs` type and the `notifEnabled` / `teamEventRadio` functions in the profile page to read `prefs[key].policy` and `prefs[key].source`.
- Rewrite `notification_prefs_test.go` to match the current API shape.

---

## High — should fix before merge

### Dead `sendTestEmail` function

`api/internal/notifications/send.go` (~line 483–505) defines `sendTestEmail` (unexported), which is never called. The handler (`me.go`) now uses `RenderTestEmail` + direct `Send` instead. The old function also constructs an inline HTML body using `html.EscapeString` on plain text that should not be escaped. Delete it.

---

## Medium

### `EventBookingAnyCreated` is orphaned

Declared in `api/internal/notifications/prefs.go`, present in `AllEvents` and `BroadcastSystemDefaults` (defaulting to off), but there is no `SendBookingAnyCreated` function and it is never triggered. It will appear in the prefs UI with no effect. Wire it up or remove it from `AllEvents`.

### Reminder notifications have no deduplication guard

`sendReminderForBooking` in `api/internal/notifications/scheduler.go` calls `sendTo` without first calling `HasNotificationBeenSent`. A server restart at the exact reminder window can double-send the email. Either add the check (mirroring the overdue-alert path) or document the limitation with a comment.

### Migrations 00009 and 00010 have no `-- +goose Down` section

`goose down` will error on these migrations. Migration 00009 also drops columns that only exist in this branch, making a Down non-trivial to write, but the section should at minimum exist as a stub with a comment explaining why it is a no-op.

---

## Low — cleanup

### `timeNow` is declared but never used

`api/internal/notifications/template.go` declares `var timeNow = time.Now` (a test-hook pattern) but the variable is never referenced anywhere in the package. `scheduler.go` calls `time.Now()` directly. Delete it or use it.

### Two unused parameters in `ResolvePrefs`

`api/internal/notifications/prefs.go`: `ResolvePrefs` accepts `channels []string` (explicitly discarded with `_`) and `isManager bool` (never referenced in the body). Both are leftovers from an earlier design. Remove them and update the single call site in `notification_prefs.go`.

### Unchecked `LogNotification` error in scheduler.go

One call to `q.LogNotification(...)` in `api/internal/notifications/scheduler.go` does not assign the error. All other call sites use `_ = q.LogNotification(...)`. Make it consistent.

### Empty committed file `web/gchat-manager-guide.md`

A zero-byte file was accidentally committed. Note: there is also a `docs/gchat-manager-guide.md` which may be the intended location. Remove the `web/` copy.

---

## Not an issue

- **i18n**: `en.json` and `sv.json` have identical key sets — all new notification keys are present in both languages.
- **Security**: HTML escaping is applied consistently throughout email rendering. SMTP and GChat credentials are encrypted at rest. All management endpoints are guarded with `RequireRole("equipment_manager")`. CTA URLs are constructed from server config + UUIDs, not user input.
- **Migration safety**: all new `NOT NULL` columns include `DEFAULT` values, safe to apply against live tables with existing rows.
- **Go context handling**: fire-and-forget notification goroutines correctly use `context.Background()` rather than the request context.
- **Team notification email redirect**: any team member can set `notification_email` to an external address — confirmed intentional per code comment.
