# Pre-release checklist

Work to complete before moving from `/api/v0/` (pre-release) to v1.0.

---

## Notifications

- [x] Schema cleanup (`00009_gchat.sql`) — drop stale webhook/channel columns
- [x] Phase 3.5a — three-tier prefs, team broadcast email, email threading
- [x] Phase 3.5b — Google Chat bot, team–space mapper UI
- [x] Phase 3.6 — Gruppkanal, personal email policy
- [x] Phase 3.7 — GChat two-message threading, issue broadcast parity
- [ ] End-to-end smoke test in dev with real GChat space (3.7-5)
- [ ] Integration tests for GChat key management endpoints (3.5b-6)

---

## Other frontend gaps

- [ ] Web header logo — fetch `logo_url` from group settings and render in top nav when present

---

## Phase 2 remaining (inventory management)

Lower priority but useful before real users arrive.

- [ ] CSV import two-phase flow — dry-run preview with per-row duplicate detection, then confirm
- [ ] CSV export — client-side, import-compatible columns, on browse page
- [ ] Print-friendly booking fetch list — `@media print`, grouped by location, on booking detail

---

## Release gate

- [ ] All items above complete
- [ ] Smoke tests pass on a clean `docker compose up` + seed
- [ ] API moved from `/api/v0/` to `/api/v1/`
- [ ] `init-group` run on production VPS
- [ ] Release Please PR merged → v1.0.0 tag + Docker images pushed
