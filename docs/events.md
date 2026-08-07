# Event relay (Phase 2)

The Catalog module writes domain events into the `outbox_events` table in the
same transaction as each state change (the transactional outbox pattern). The
relay turns those rows into a NATS JetStream event stream.

## How it works

1. The relay polls `outbox_events` for rows where `published_at IS NULL`, using
   `SELECT ... FOR UPDATE SKIP LOCKED` so multiple relay replicas never claim
   the same row.
2. For each claimed row it publishes to subject
   `forge.events.<event_type>` (for example
   `forge.events.catalog.service.registered`), setting the JetStream message id
   to the outbox row id.
3. It marks the rows `published_at = now()` and commits.

## Delivery guarantees

Delivery is at least once. If the process crashes after publishing but before
committing, the transaction rolls back and the rows are re-published on the next
tick. The stream's deduplication window (message id = outbox row id) collapses
those re-publishes to a single stored message, so consumers see each event once
per window. Consumers must still be written to tolerate redelivery.

## Running locally

```bash
docker compose up -d postgres nats
make run        # terminal 1: API (writes the outbox)
make relay      # terminal 2: relay (publishes the outbox)
make consumer   # terminal 3: example consumer (logs each event)
```

Then register a service through the API; within a second the consumer logs the
`catalog.service.registered` event. Every lifecycle and ownership change flows
the same way.

## Configuration

| Variable | Default | Purpose |
|---|---|---|
| FORGE_NATS_URL | nats://localhost:4222 | NATS server URL |
| FORGE_EVENTS_STREAM | FORGE_EVENTS | JetStream stream name |
| FORGE_EVENTS_SUBJECT_PREFIX | forge.events | Subject root for all events |
| FORGE_RELAY_BATCH_SIZE | 100 | Rows claimed per poll |
| FORGE_RELAY_POLL_INTERVAL | 1s | Poll cadence |
