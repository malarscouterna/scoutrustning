# Pre-release checklist

Work to complete before moving from `/api/v0/` (pre-release) to v1.0.

---

## Schema cleanup (bundle into 00009_gchat.sql)

Drop stale columns that are vestigial pre-launch. Doing this now avoids them being permanently embedded in production backups.

- [x] Drop `users.notification_channel` — superseded by `users.notification_prefs` (jsonb); never read by dispatch logic
- [x] Drop `users.gchat_webhook_url` — never exposed in UI, replaced by service-account-based GChat in 3.5b
- [x] Drop `teams.gchat_webhook_url` — never exposed in UI, replaced by `teams.gchat_space_id` in 3.5b
- [x] Drop `group_settings.gchat_webhook_url` — replaced by `gchat_service_account_json_encrypted` + bot approach in 3.5b

---

## Notifications Phase 3.5a — frontend remaining

Backend fully done (steps 1–6). Frontend work only.

- [ ] **3.5a-7** Team notification settings UI on `/teams/[id]` (any team member)
  - Accessible to all team members — not manager-only. Groups cannot guarantee every team has a trusted member, and these settings only affect the team itself.
  - Backend: relax `GET/PUT /teams/{id}/notification-settings` from `equipment_manager` to a membership check (caller's `team_ids` contains the team ID, or is a group manager).
  - Broadcast email input (`notification_email`)
  - "Suppress individual notifications" toggle (`individual_notifications_enabled`)
  - Team notification defaults table (same layout as group defaults, columns driven by `enabled_channels`)
- [ ] **3.5a-8** Update `GET /me/notification-prefs` `source` field to surface `"team_default"`; add frontend label "(teamstandard)"
- [ ] **3.5a-9** "Återställ alla användares notiser till standard" button in group settings notification section, behind confirmation dialog (`POST /api/v0/group-settings/force-notification-defaults`)

---

## Notifications Phase 3.5b — Google Chat bot

Full design in [notifications-phase35.md](notifications-phase35.md). This phase includes the `00009_gchat.sql` migration where the schema cleanup above also lives.

- [x] **3.5b-1** Migration `00009_gchat.sql`
  - Add `group_settings.gchat_service_account_json_encrypted` (bytea)
  - Add `group_settings.gchat_admin_email` (text)
  - Add `teams.gchat_space_id` (text)
  - Drop stale columns listed in schema cleanup section above
- [x] **3.5b-2** `GChatNotifier` — card builder, email-style threading via `thread.threadKey`, DWD service account auth
- [x] **3.5b-3** GChat key endpoints
  - `POST /api/v0/group-settings/gchat-key` — validate key, store encrypted, append `"gchat"` to `enabled_channels`
  - `DELETE /api/v0/group-settings/gchat-key` — remove key, remove `"gchat"` from `enabled_channels`, clear all `teams.gchat_space_id`
- [ ] **3.5b-4** Team–Space mapper UI in group settings (Integrationer section)
  - [x] `GET /api/v0/group-settings/gchat-spaces` — list accessible spaces
  - [x] `PUT/DELETE /api/v0/teams/{id}/gchat-space` — link/unlink space; bot auto-added on link, welcome card sent
  - [ ] Frontend UI (group settings › Integrationer section)
- [x] **3.5b-5** Dispatch loop — gchat broadcast path (parallel to email broadcast in 3.5a-4)

---

## Other frontend gaps

- [ ] Web header logo — fetch `logo_url` from group settings and render in top nav when present (backend done, frontend not wired up)

---

## Phase 2 remaining (inventory management)

Lower priority but useful before real users arrive.

- [ ] CSV import two-phase flow — dry-run preview with per-row duplicate detection, then confirm. Needs `import_batches` table for revertability.
- [ ] CSV export — client-side, import-compatible columns, on browse page
- [ ] Print-friendly booking fetch list — `@media print`, grouped by location, on booking detail

---

## Release gate

- [ ] All items above complete
- [ ] Smoke tests pass on a clean `docker compose up` + seed
- [ ] API moved from `/api/v0/` to `/api/v1/`
- [ ] `init-group` run on production VPS
- [ ] Release Please PR merged → v1.0.0 tag + Docker images pushed
