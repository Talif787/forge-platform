# Architecture (implementation notes)

This document covers implementation-level architecture. The full system design
is in ARCH-FORGE-001; the requirements are in SRD-FORGE-001.

## Layering (per module)

Hexagonal (ports and adapters). Dependencies point inward:

```
api (handlers) -> app (use cases) -> domain (rules, no I/O)
adapters (postgres, ...) implement ports defined by app
```

The domain layer has no outbound dependencies and is fully unit-testable. The
application layer orchestrates use cases against port interfaces. Adapters are
the only place external technology appears.

## Consistency and events

Writes run inside a unit of work (`Atomic`) that spans the repository and the
outbox, so the aggregate and its domain events commit in one transaction. A
relay (Phase 2) publishes outbox rows to the event bus at least once; consumers
are idempotent.

## Concurrency

Aggregates carry a version. Writes use `UPDATE ... WHERE version = expected`;
zero rows affected means either the row is gone (404) or was modified
concurrently (409). Clients pass the expected version via `If-Match` and receive
the new version via `ETag`.

## Pagination

Keyset (seek) pagination over `(sort_column, id)` with an opaque base64 cursor
that encodes the sort field, order, boundary value, and id. This is stable and
constant-time regardless of page depth, unlike offset pagination.

## Time-versioned ownership

Ownership changes append history rows with validity intervals rather than
overwriting, so cost and audit can resolve the owner as of any past instant.
