# ms-utrustning

Equipment booking service for scout groups. Book tents, stoves, and other gear — track inventory, handle pickups and returns with checklists, report issues.

See [docs/SPEC.md](docs/SPEC.md) for the full specification.

## Stack

- **API**: Go 1.26 (Chi v5, pgx v5, sqlc)
- **Frontend**: SvelteKit 2 + Svelte 5 + @scouterna/ui-webc 3
- **Styling**: Tailwind CSS 4 + @scouterna/tailwind-theme
- **Database**: PostgreSQL 17
- **Auth**: Auth.js (@auth/sveltekit) with Keycloak OIDC, JWT validation in Go API
- **Migrations**: goose v3
- **Deployment**: Docker Compose

## Development

```bash
# Start everything
docker compose up

# API only (requires Go 1.26+)
cd api && go run ./cmd/server

# Frontend only (requires Node 24+, pnpm)
cd web && pnpm dev
```

## Environment variables

See [.env.example](.env.example).
