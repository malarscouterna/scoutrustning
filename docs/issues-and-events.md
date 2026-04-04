# Issue Reporting & Article Events

## Overview

Issue reporting is built on top of article statuses and article events — no separate `issue_reports` table. An article with status `reported_usable` or `reported_unusable` has an open issue. The `article_events` table provides the full per-article audit trail.

## How it works

### Reporting an issue

Any user can report an issue via `POST /api/v0/articles/{id}/report`. This:
1. Sets the article status to `reported_usable` or `reported_unusable` based on severity
2. Creates an `issue_reported` article event with the description

### Managing issues

Equipment managers see articles with reported/repair statuses on the Ärenden page. They can:
- **Update status** (`PUT /api/v0/articles/{id}/status`) — change to `under_repair`, back to a reported state, `ok` (resolved), or `archived`. Each update creates an article event with an optional comment.
- When status is set to `ok` from a non-ok state, the event type is `issue_resolved` instead of `status_change`.

### Issue lifecycle (via article status)

```
ok → reported_usable/reported_unusable (user reports issue)
   → under_repair (manager acknowledges, sends to repair)
   → ok (manager marks as resolved)
   → archived (manager retires article)
```

Any transition is valid — a manager can go directly from `reported_unusable` to `ok`, or from `under_repair` back to `reported_usable` if the repair didn't work.

### Auto-created issues from returns

When a booking item is returned as `broken`, the article status is set to `reported_unusable` and an `issue_reported` event is logged with the return notes. When returned as `lost`, the article is set to `archived`.

## Article events

The `article_events` table logs everything that happens to an article:

| Event type | When it's logged |
|---|---|
| `issue_reported` | User reports issue, or item returned as broken/lost |
| `issue_resolved` | Manager sets status back to `ok` from a non-ok state |
| `status_change` | Manager changes status (under_repair, archived, etc.) |
| `returned` | Item returned OK via booking return flow |

Events not yet emitted (future):
- `booked` — when assigned to a booking
- `picked_up` — when picked up
- `note` — free-form manager notes

## Design decisions

### Why no `issue_reports` table?

The article status already tells us if there's an open issue (`reported_usable`, `reported_unusable`). The manager queue is just a filtered article list. The narrative (who reported what, what actions were taken) lives in article events. A separate table would duplicate state and require keeping it in sync.

### Why fire-and-forget for event logging?

Event logging never blocks the primary operation. If an event fails to log, the user's action still succeeds.

## Future work

- Image attachments on issue reports (Phase 2)
- `reported_usable` articles assigned last during booking, with a warning in the UI
- Richer event logging (`booked`, `picked_up`, `note` event types)
