# Local development

## Prerequisites

- Go 1.23+
- Docker (for `docker compose` and integration tests)
- `migrate` CLI (optional; migrations also run automatically at startup)

## Run

```bash
cp .env.example .env
docker compose up --build          # Postgres + API
# or, against a local Postgres:
make run
```

## Authentication in dev

The default auth mode is `hmac` (HS256). Generate a token signed with
`FORGE_AUTH_HMAC_SECRET` containing at least `sub`, and optionally `email` and
`groups`. Any JWT library or `jwt.io` works for local testing. Production uses
`jwks` against the organizational IdP.

## Tests

```bash
make test-unit           # no Docker required
make test-integration    # spins Postgres via testcontainers; needs Docker
```

## Common tasks

- Add a migration: create `migrations/000N_description.up.sql` and a matching
  `.down.sql`. It applies automatically on next startup, in version order.
- Regenerate nothing: types are hand-written. The OpenAPI spec in
  `api/openapi.yaml` is the API contract of record.
