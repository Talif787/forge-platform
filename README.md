# Forge Control Plane

Reference implementation of the Forge Internal Developer Platform control plane.
This repository accompanies the design documents SRD-FORGE-001 (requirements)
and ARCH-FORGE-001 (architecture). It is built in phases; this is Phase 1.

## Phase 1 scope

A complete, independently buildable vertical slice: the platform API for the
**Catalog** bounded context (the service registry), implemented through every
layer of the architecture. It establishes the pattern every other module
follows. Later phases add the remaining modules, the reconciliation
controllers, the durable provisioning workflows, and the delivery pipeline.

Delivered here:

- Service registry with lifecycle (experimental, production, deprecated,
  retired), criticality tiers, and time-versioned ownership.
- REST API with versioning, pagination (keyset), filtering, sorting,
  idempotency, optimistic concurrency (If-Match / ETag), and a consistent
  error envelope.
- Clean Architecture: domain, application, adapters, and interface layers with
  the dependency rule enforced by direction.
- Transactional outbox for reliable domain-event emission.
- JWT authentication (HMAC for local development, JWKS/RS256 for OIDC),
  per-identity rate limiting, structured JSON logging with correlation ids,
  OpenTelemetry tracing, Prometheus metrics, and liveness/readiness probes.
- Postgres persistence with an embedded migration runner.
- Unit, application, API, and testcontainers-backed integration tests.

## Quickstart

```bash
cp .env.example .env
docker compose up --build
```

The API listens on `:8080`. Mint a local HS256 token (the default dev auth mode
is `hmac`) with subject and groups claims, then:

```bash
curl -s localhost:8080/healthz
curl -s -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: $(uuidgen)" \
  -d '{"tenantId":"00000000-0000-0000-0000-000000000001","name":"payments-api","tier":1,"owningTeam":"payments","onCallRef":"pd://payments"}' \
  localhost:8080/api/v1/services
```

## Local development

```bash
make run            # run against a local Postgres (see .env)
make test-unit      # unit + application + API tests (no Docker)
make test-integration  # repository tests via testcontainers (needs Docker)
make vet
```

Migrations apply automatically at startup. The Makefile also exposes the
`migrate` CLI targets for manual control.

## Project structure

```
cmd/api                     Composition root and process lifecycle
internal/platform           Cross-cutting: config, logging, errors, ids,
                            observability, postgres, httpx, idempotency
internal/modules/catalog
  domain                    Aggregate, value objects, domain events, rules
  app                       Use cases (commands, queries) and ports
  adapters/postgres         Repository, outbox, unit-of-work
  api                       DTOs, handlers, routes
  module.go                 Dependency wiring for the bounded context
migrations                  SQL migrations (embedded)
api/openapi.yaml            OpenAPI 3.1 specification
docs                        Configuration, local dev, runbook
```

The dependency rule: `api` and `adapters` depend on `app`; `app` depends on
`domain`; `domain` depends on nothing outside the platform primitives. A module
never imports another module's internals. This keeps business logic testable
without infrastructure and lets the substrate implementation change without
touching a domain rule.

## Key implementation decisions

- **Catalog first.** It is central, rich in domain rules (lifecycle
  transitions, on-call-before-production, time-versioned ownership), and
  testable without standing up Kubernetes.
- **Modular monolith core.** One consistency boundary, one team; process
  boundaries are drawn where the domain has real seams, not by default.
- **Optimistic concurrency** over locking, so contention degrades to a retriable
  409 rather than blocking.
- **Keyset pagination** over offset, for stable performance at fleet scale.
- **Transactional outbox** so events commit atomically with state and are never
  emitted for a rolled-back change. A bus relay is Phase 2.
- **Reference-and-inject auth strategy** (HMAC vs JWKS) selected by config; HMAC
  is rejected in production by config validation.
- **In-memory rate limiting** for a single instance now; a Redis-backed limiter
  replaces it when the control plane runs multiple replicas.

## Testing

- `domain` tests exercise invariants and lifecycle transitions with no I/O.
- `app` tests exercise use cases against in-memory fakes.
- `api` tests exercise the HTTP surface end to end with an in-memory store.
- `adapters/postgres` integration tests run against a real Postgres via
  testcontainers, behind the `integration` build tag.
