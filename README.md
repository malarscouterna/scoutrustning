# scoutrustning

Equipment booking service for scout groups. Leaders book scouting gear (tents, stoves, knives, etc.), pick it up with a checklist, and return it. Equipment managers run the inventory, approve bookings, and handle issue reports.

Built for Mälarscouterna and designed from the start to support multiple scout groups in the same deployment.

See [docs/SPEC.md](docs/SPEC.md) for the full specification and [docs/API.md](docs/API.md) for the API reference.

## What it does

A leader visits the site, picks dates, browses available gear, adds items to a cart, and submits. If the gear requires approval, a manager reviews it; otherwise the booking auto-confirms. At pickup, the leader works through a checklist that shows which specific items to collect and where to find them. On return, each item is ticked off individually — broken or missing items trigger an issue report. Any user can file an issue at any time; managers track and resolve them.

Managers get a full inventory UI: create and edit articles, bulk-move items between locations, manage categories and locations, upload product images, approve or reject bookings, and handle the issue queue.

## Key concepts

**Articles** come in two kinds. *Individually tracked* items each have their own name and identity (e.g. "Sibley 1", "Sibley 2") — the pickup checklist tells you exactly which one to grab and where. *Quantity-tracked* items are interchangeable units of the same type (e.g. "Tältlampa LED") where you just grab the right count. Both are stored the same way internally; the UI groups and presents them differently.

**Article status** describes physical condition (`ok`, `reported_usable`, `reported_unusable`, `under_repair`, `incoming`, `archived`) and is separate from booking state (reserved, loaned out, returned). Availability is always computed from condition + overlapping bookings — never stored as a column. This means the same item can be "available now, reserved next week, available again in August."

**Approval levels** control whether a booking needs manager review. Each article has a level: `none` (freely bookable), `low` (trusted teams auto-approve, others need a manager), or `high` (always needs a manager, even managers booking for themselves). If any item in a booking requires approval for that team, the whole booking waits. When waiting for approval, all items are reserved in the system.

**Teams** are the unit of access control. Every team has an access level: `view` (browse and report issues only), `book` (standard), `trusted` (auto-approves `low` items), `manager` (full access). Teams come from OIDC claims at login and are auto-created on first login, or pre-created by managers. The access level is configurable per team.

**Issues** are tracked as their own objects, not just a flag on an article. Any user can file an issue on any article from the browse page, the article detail page, or the return checklist. Each issue has a severity (`usable` — still bookable but flagged; `unusable` — taken out of circulation; `missing`), a lifecycle (`open` → `in_progress` → `resolved`), assignees, a comment thread, and optional image attachments. Article condition is derived from open issues: the worst-severity open issue determines the article's status, and when all issues are resolved the article automatically returns to `ok`.

**Images** are stored per product type and location — all physical articles of the same type at the same location share one image gallery. Who can upload is configurable per group (any user, leaders only, trusted, or managers only). Uploaders use a crop UI with selectable format (landscape, portrait, or square) and can add photographer attribution. Images can be marked as *shared*, making them browsable by equipment managers in other groups who can then reuse the same photo for their own inventory without re-uploading.

## Scope and limitations

The system is built around the way Swedish scout organizations are structured. A few things reflect that context and would need work before the service is usable outside it:

- **Authentication is ScoutID-only.** Login goes through Keycloak with a specific claim format used by Scoutnet (`group:766:material_responsible`, `troop:17443:vice_leader`). There is no local username/password option. Deploying for a non-Swedish-scout organization would require either connecting to a compatible Keycloak/OIDC provider or adding an alternative auth path.
- **UI components are from @scouterna/ui-webc.** The design system (`@scouterna/ui-webc`, `@scouterna/tailwind-theme`) is the Swedish scouting design system. The components work well but carry Scouternas visual identity. Swapping them out is possible but non-trivial.
- **The UI is Swedish-first.** Swedish is the default language and all user-visible strings exist in Swedish. English is fully supported and can be set per user or per group, but the default experience is Swedish. The interface may use terms that make sense only in a Swedish scout environment.

The data model and multi-tenancy architecture are not tied to Sweden — multiple groups with different access configurations can run in the same deployment — but the auth and UI layers reflect the original context.

## What's implemented

- Full booking flow: draft → submit → approve/reject → confirm → pickup checklist → return checklist
- Three-level approval with per-team access levels and approval conversation history
- Double-booking prevention (availability computed at query time, not stored)
- Pickup checklist with swap support; partial returns; issue reporting on return
- Inventory management: article CRUD, bulk status/location changes, quantity-tracked group editing, shared field propagation
- Issue reports with event history, assignees, comments, and image attachments
- Product images with crop UI, gallery viewer, shared image browser, and photographer attribution
- Group settings: locations, categories, CSV import, SMTP, approval defaults, language
- Notification preferences: per-user and per-group defaults with event-level granularity (email column; channel infrastructure supports more)
- OIDC login via ScoutID (Keycloak); JWT validation in Go; teams auto-created from claims
- Swedish and English UI; language switchable per user or group
- Multi-tenancy: every table is scoped by `group_id`; multiple groups can share a deployment

### What's not yet done

- **CSV import preview** — import runs immediately with no dry-run step.
- **CSV export and print view** — no export from the browse page; no print-friendly checklist.
- **Packages** — predefined article sets that populate the cart (designed but not built).
- **Playwright E2E tests** — integration tests and smoke tests exist; browser automation is not set up.

## Stack

- **API**: Go 1.26 (Chi v5, pgx v5, sqlc, govips)
- **Frontend**: SvelteKit 2 + Svelte 5 + @scouterna/ui-webc 3
- **Styling**: Tailwind CSS 4 + @scouterna/tailwind-theme
- **Database**: PostgreSQL 17
- **Auth**: Auth.js (@auth/sveltekit) with Keycloak OIDC, JWT validation in Go
- **Migrations**: goose v3

## Development

```bash
# Generate .env for local development
./gen-env.sh dev

# Start everything with hot reload (Go API + SvelteKit + Postgres)
docker compose up

# In another terminal, seed the database
./dev-seed.sh
```

In dev mode, no login is required. Use the persona switcher (floating panel) to switch between preconfigured roles. See `dev-personas.json` for available personas.

### Testing email notifications in dev

[Mailpit](https://mailpit.axllent.org/) is included in the dev Compose stack — it starts automatically with `docker compose up`. It catches all outgoing email and shows it in a web UI at `http://localhost:8025`. No configuration needed; the generated `.env` already points at it (`SMTP_DEFAULT_HOST=mailpit`).

To test against a real provider instead, set `SMTP_DEFAULT_*` in `.env` to your mail provider credentials (SendGrid, Mailgun, a Gmail app password, etc.) and restart the API container.

Use the **Skicka testnotis** button on your profile page (group settings tab) to send a test email without triggering a booking event.

Booking events (confirmed, rejected, etc.) trigger emails automatically during `./dev-seed.sh`. Check Mailpit at `http://localhost:8025` after seeding.

### Testing Google Chat notifications in dev

To test GChat notifications locally you need a Google Workspace service account with Domain-Wide Delegation and an existing Chat Space.

1. Place the service account JSON file at `dev-secrets/gchat-key.json` (gitignored).
2. Fill in the three vars in `.env`:
   ```
   DEV_GCHAT_KEY_PATH=./dev-secrets/gchat-key.json
   DEV_GCHAT_SPACE_ID=spaces/XXXXXXXXX
   DEV_GCHAT_ADMIN_EMAIL=admin@yourorg.com
   ```
3. Re-run `./dev-seed.sh` — it will upload and validate the key, then link **Yggdrasil** (email + GChat) and **Utrustningsgruppen** (GChat only) to the space. Subsequent booking events in the seed will trigger GChat notifications to that space.

**What reaches the space**: booking events (`booking_confirmed`, `booking_needs_approval`, `booking_reminder`, etc.) for Yggdrasil bookings. Issue events do not yet have a GChat broadcast path (known gap — see `docs/notifications-phase35.md`).

You still need `docker compose up --build` when adding Go or Node dependencies, or changing a Dockerfile.

### Running tests

**Prerequisites**: libvips is required for image processing tests.
```bash
sudo apt install libvips-dev
```

```bash
cd api && go test ./internal/handler/tests/ -timeout 180s -count=1 2>&1 && bash ../smoke-test.sh
```

API tests use testcontainers-go against a real Postgres instance. The smoke test curls every page through SvelteKit and checks for 500 errors.

### Environment modes

| | Dev | Demo | Production |
|---|---|---|---|
| `DEV_MODE` | `true` | `true` | `false` |
| `DEMO_MODE` | `false` | `true` | `false` |
| Hot reload | yes | no | no |
| Login required | no | yes (ScoutID) | yes (ScoutID) |
| Persona switcher | yes | yes (post-login) | no |

Generate `.env` for any mode with `./gen-env.sh dev|demo|prod`. Switch modes with `./gen-env.sh <mode> --force && docker compose up -d --build`.

## Deployment

Requires Docker, Docker Compose, a reverse proxy (nginx, Caddy, Traefik) for TLS, and a Keycloak client. The reverse proxy forwards to the SvelteKit container (port 3000). The Go API port (8080) is bound to localhost only and never exposed directly.

```bash
./gen-env.sh prod
# Fill in ORIGIN and AUTH_KEYCLOAK_SECRET in .env
docker compose pull && docker compose up -d
docker compose exec api /bin/server init-group \
  --group-id YOUR_ORG_ID --group-name "Your Scout Group" \
  --manager-claim "group:YOUR_ORG_ID:material_responsible" \
  --team-name "Equipment Managers"
./dev-seed.sh path/to/your-inventory.csv
```

Updates: `docker compose pull && docker compose up -d`. Migrations run automatically on startup.

### Multiple domains

The service can be reached on multiple domains simultaneously (e.g. `utrustning.malarscouterna.se` and `scoutrustning.se` pointing to the same instance). The reverse proxy should rewrite the `Origin` header to the canonical domain before forwarding to SvelteKit, so that `ORIGIN` in `.env` stays a single value and CSRF protection works correctly regardless of which domain the user hit.

```caddyfile
utrustning.example.com, scoutrustning.se {
    reverse_proxy localhost:3000 {
        header_up Origin "https://utrustning.malarscouterna.se"
    }
}
```

Set `ORIGIN=https://utrustning.malarscouterna.se` in `.env`. Caddy handles TLS for both domains automatically.

For full deployment details, security model, and reverse proxy setup see the [Deployment section in the old README](docs/SPEC.md) or `docker-compose.yml` and `gen-env.sh`.

## License

Copyright 2025 Teo Elmfeldt

Licensed under the [MIT License](LICENSE).

## Contributing

All contributions must include a [Developer Certificate of Origin](DCO) sign-off to certify that you have the right to submit the code under the MIT license:

```bash
git commit -s -m "feat: add new feature"
```

PRs with unsigned commits will not be accepted. AI-assisted contributions are welcome; you are responsible for what you submit.
