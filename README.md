# ms-utrustning

Equipment booking service for scout groups. Book tents, stoves, and other gear — track inventory, handle pickups and returns with checklists, report issues.

See [docs/SPEC.md](docs/SPEC.md) for the full specification and [docs/API.md](docs/API.md) for the API reference.

## Status

Pre-release (`v0`). Breaking changes expected. Currently implements:
- Article inventory with CSV import
- Article browsing with category/location/search filters
- Booking flow: create, add items, submit, cancel, copy
- Availability calculation with double-booking prevention
- Location-scoped availability (same product in different locations shown separately)
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
# Start everything (builds and runs Go API + SvelteKit + Postgres)
docker compose up --build

# Import inventory (first time)
curl -X POST http://localhost:8080/api/v0/articles/import \
  -H "X-Dev-Role-Override: equipment-manager" \
  -F "file=@path/to/inventory.csv"

# Create units
curl -X POST http://localhost:8080/api/v0/units \
  -H "X-Dev-Role-Override: equipment-manager" \
  -H "Content-Type: application/json" \
  -d '{"name":"Yggdrasil"}'
```

In dev mode (`DEV_MODE=true`), use the `X-Dev-Role-Override` header to switch personas. See `dev-personas.json` for available personas.

### Running tests

```bash
cd api && go test ./internal/handler/tests/ -timeout 180s
```

Tests use testcontainers-go (requires Docker) and run against a real Postgres instance.

## Environment variables

See [.env.example](.env.example).
