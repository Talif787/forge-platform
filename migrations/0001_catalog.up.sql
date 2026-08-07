CREATE TABLE catalog_services (
    id             UUID PRIMARY KEY,
    tenant_id      UUID NOT NULL,
    name           TEXT NOT NULL,
    description    TEXT NOT NULL DEFAULT '',
    tier           SMALLINT NOT NULL CHECK (tier BETWEEN 1 AND 4),
    lifecycle      TEXT NOT NULL CHECK (lifecycle IN ('experimental','production','deprecated','retired')),
    repository_url TEXT NOT NULL DEFAULT '',
    owning_team    TEXT NOT NULL,
    on_call_ref    TEXT NOT NULL DEFAULT '',
    version        BIGINT NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL,
    updated_at     TIMESTAMPTZ NOT NULL,
    CONSTRAINT catalog_services_tenant_name_unique UNIQUE (tenant_id, name)
);

CREATE INDEX catalog_services_tenant_idx ON catalog_services (tenant_id);
CREATE INDEX catalog_services_lifecycle_idx ON catalog_services (lifecycle);
CREATE INDEX catalog_services_created_at_id_idx ON catalog_services (created_at DESC, id DESC);

-- Ownership is time-versioned so historical cost and audit can resolve the
-- owner as of any past instant, rather than only the current owner.
CREATE TABLE catalog_ownership_history (
    id          UUID PRIMARY KEY,
    service_id  UUID NOT NULL REFERENCES catalog_services (id) ON DELETE CASCADE,
    owning_team TEXT NOT NULL,
    on_call_ref TEXT NOT NULL DEFAULT '',
    valid_from  TIMESTAMPTZ NOT NULL,
    valid_to    TIMESTAMPTZ
);

CREATE INDEX catalog_ownership_history_asof_idx
    ON catalog_ownership_history (service_id, valid_from DESC);

-- Transactional outbox: domain events are written in the same transaction as
-- the state change and relayed to the event bus asynchronously (Phase 2+).
CREATE TABLE outbox_events (
    id            UUID PRIMARY KEY,
    aggregate_id  UUID NOT NULL,
    event_type    TEXT NOT NULL,
    payload       JSONB NOT NULL,
    occurred_at   TIMESTAMPTZ NOT NULL,
    published_at  TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX outbox_events_unpublished_idx
    ON outbox_events (created_at) WHERE published_at IS NULL;

CREATE TABLE idempotency_keys (
    id              UUID PRIMARY KEY,
    idem_key        TEXT NOT NULL,
    request_hash    TEXT NOT NULL,
    response_status INT NOT NULL,
    response_body   JSONB NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT idempotency_keys_unique UNIQUE (idem_key)
);
