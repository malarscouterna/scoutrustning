# ms-utrustning

Equipment booking service for scout groups. Book tents, stoves, and other gear — track inventory, handle pickups and returns with checklists, report issues.

See [docs/SPEC.md](docs/SPEC.md) for the full specification and [docs/API.md](docs/API.md) for the API reference.

## Status

Pre-release (`v0`). Breaking changes expected. Currently implements:
- Article inventory with CSV import (including approval level per article)
- Article browsing with category/location/search filters
- Booking flow: create, add items, submit, cancel, copy
- Three-level approval: none (free), low (project leaders auto-approve), high (always needs manager)
- Approval conversation thread with booking events (submit/reject/resubmit/approve with messages)
- Force-approval option for leaders who want manager review on freely bookable items
- Availability calculation with double-booking prevention
- Location-scoped availability (same product in different locations shown separately)
- Pickup and return checklists with swap support
- Issue reporting with per-article event history
- Role-based access (leader, project leader, equipment manager)
- Multi-tenancy (group-scoped, ready for multiple organizations)

## Stack

- **API**: Go 1.26 (Chi v5, pgx v5, sqlc) — `/api/v0/*`
- **Frontend**: SvelteKit 2 + Svelte 5 + @scouterna/ui-webc 3
- **Styling**: Tailwind CSS 4 + @scouterna/tailwind-theme
- **Database**: PostgreSQL 17
- **Auth**: Auth.js (@auth/sveltekit) with Keycloak OIDC, JWT validation in Go API
- **Migrations**: goose v3
- **Deployment**: Docker Compose

## Development

```bash
# Generate .env for local development
./gen-env.sh dev

# Start everything with hot reload (Go API + SvelteKit + Postgres)
docker compose up

# In another terminal, seed the database (import inventory + create units)
./dev-seed.sh
```

Code changes auto-reload:
- **Go API**: [air](https://github.com/air-verse/air) watches `.go` and `.sql` files, rebuilds and restarts (~1-2s)
- **SvelteKit**: Vite dev server with HMR (near-instant)

You still need `docker compose up --build` when:
- Adding new Go or Node dependencies
- Changing a Dockerfile

The seed script imports from `docs/Utrustningsregister MS.xlsx - data.csv` by default. Pass a different path as an argument:
```bash
./dev-seed.sh path/to/other.csv
```

In dev mode (`DEV_MODE=true`), use the `X-Dev-Role-Override` header to switch personas. See `dev-personas.json` for available personas.

### Running tests

```bash
# Full test suite (requires Docker for testcontainers + docker compose up + ./dev-seed.sh)
cd api && go test ./internal/handler/tests/ -timeout 180s -count=1 2>&1 && bash ../smoke-test.sh
```

API tests use testcontainers-go and run against a real Postgres instance. A single shared container is reused across all tests for speed.

The smoke test curls every page through SvelteKit and asserts no 500 errors. Catches SSR crashes that are silent on client-side navigation but break on page reload.

## Environment modes

All differences between dev, demo, and production are controlled via `.env`. Generate it with:

```bash
./gen-env.sh dev    # Local development
./gen-env.sh demo   # Demo deployment
./gen-env.sh prod   # Production
```

Flags:
- `--force` — overwrite existing `.env` (preserves user-edited values like `ORIGIN` and `AUTH_KEYCLOAK_SECRET`)
- `--local` — use local image names, localhost origin, and static Postgres password (for testing demo/prod modes on a dev machine). No effect on `dev` mode which is already local.

| | Dev | Demo | Production |
|---|---|---|---|
| `DEV_MODE` | `true` | `true` | `false` |
| `DEMO_MODE` | `false` | `true` | `false` |
| `BUILD_TARGET` | `dev` | `production` | `production` |
| `AUTH_SECRET` | static | generated | generated |
| `POSTGRES_PASSWORD` | static | generated | generated |

- **Dev**: hot reload, persona switcher, auto-fallback to default persona (no login required), Postgres exposed
- **Demo**: production builds, OIDC login required (ScoutID), persona switcher available after login, demo banner shown
- **Production**: production builds, OIDC login required, no persona switcher, no demo banner

The generated `.env` includes a `COMPOSE_FILE` variable that controls which compose files are loaded. Dev includes `docker-compose.override.yml` (local builds, source mounts, Postgres port). Demo and prod use only `docker-compose.yml` (pre-built images, no source needed). Switch modes on the same machine with:

```bash
./gen-env.sh demo --force
docker compose up -d --build
```

The `--build` is needed when switching modes to rebuild images with the correct Dockerfile target. After the first build, `docker compose up -d` is enough for restarts.

### Security model

- The Go API port (8080) is bound to `127.0.0.1` — only reachable from the host machine, not from the network. The SvelteKit app proxies `/api/*` requests internally via the Docker network.
- The proxy **strips** `X-Dev-Role-Override` and `Authorization` headers from incoming browser requests before forwarding. Identity is injected server-side from the OIDC session or persona cookie — users cannot forge it.
- `DEV_MODE=true` in demo is safe because the persona override header only reaches the Go API through the SvelteKit proxy, which controls it. Direct access to the API container is blocked from the network.
- Postgres is not exposed to the host in demo/prod (no `docker-compose.override.yml`). Password is randomly generated.
- Your reverse proxy should only forward traffic to the SvelteKit port (3000). Never expose the API port (8080) directly.

## Deployment

### Prerequisites

- A VPS with Docker and Docker Compose installed
- A reverse proxy (nginx, Caddy, Traefik) handling TLS and forwarding to port 3000
- A Keycloak client configured for the app (client ID: `ms-utrustning`, redirect URI: `https://your-domain/auth/callback/keycloak`)

### Files needed on the server

You don't need the full repo. Copy these files to your deployment directory:

```
ms-utrustning/
├── docker-compose.yml
├── gen-env.sh
├── dev-seed.sh
├── dev-personas.json       # Persona definitions (needed for demo mode)
├── role-mapping.json       # Scoutnet role → app role mapping
└── docs/
    ├── import-example.csv  # Or your real inventory CSV
    └── guide.md            # User guide (shown in the UI)
```

Do **not** copy `docker-compose.override.yml` — that file enables dev-only features (local builds, source mounts, exposed Postgres port).

### Demo deployment

```bash
# 1. Generate environment
./gen-env.sh demo

# 2. Fill in the CHANGEME values in .env:
#    ORIGIN=https://your-demo-domain.example.com
#    AUTH_KEYCLOAK_SECRET=<your keycloak client secret>

# 3. Authenticate with GitHub Container Registry (one-time)
echo "YOUR_GITHUB_PAT" | docker login ghcr.io -u YOUR_GITHUB_USERNAME --password-stdin

# 4. Pull images and start
docker compose pull
docker compose up -d

# 5. Seed the database with inventory and sample bookings
./dev-seed.sh
```

### Production deployment

```bash
# 1. Generate environment
./gen-env.sh prod

# 2. Fill in the CHANGEME values in .env

# 3. Pull images and start
docker compose pull
docker compose up -d

# 4. Import your inventory
./dev-seed.sh path/to/your-inventory.csv
```

### Reverse proxy setup

The reverse proxy forwards traffic to the SvelteKit container (port 3000). If your reverse proxy runs in Docker (e.g. Caddy), the `web` service needs to be on the same Docker network. Create a `docker-compose.caddy.yml` alongside the main compose file:

```yaml
services:
  web:
    networks:
      - default
      - caddy

networks:
  caddy:
    external: true
    name: your-caddy-network-name
```

Then add it to `COMPOSE_FILE` in `.env`:

```
COMPOSE_FILE=docker-compose.yml:docker-compose.caddy.yml
```

If your reverse proxy runs directly on the host (not in Docker), no extra network config is needed — it connects to `localhost:3000` (or whatever `WEB_PORT` is set to).

### Updating

```bash
docker compose pull
docker compose up -d
```

The API runs database migrations automatically on startup. No manual migration step needed.

### Re-seeding

To reset the demo data (or re-seed after a schema change):

```bash
./dev-seed.sh
```

The seed script clears existing data before importing. It requires `DEV_MODE=true` on the API (dev and demo modes). In production (`DEV_MODE=false`), the seed script will refuse to run.
