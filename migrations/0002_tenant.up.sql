CREATE TABLE tenants (
    id           UUID PRIMARY KEY,
    name         TEXT NOT NULL,
    slug         TEXT NOT NULL,
    plan         TEXT NOT NULL CHECK (plan IN ('free','standard','enterprise')),
    status       TEXT NOT NULL CHECK (status IN ('active','suspended','archived')),
    max_services INTEGER NOT NULL CHECK (max_services >= 0),
    version      BIGINT NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL,
    updated_at   TIMESTAMPTZ NOT NULL,
    CONSTRAINT tenants_slug_unique UNIQUE (slug)
);

CREATE INDEX tenants_status_idx ON tenants (status);
CREATE INDEX tenants_created_at_id_idx ON tenants (created_at DESC, id DESC);
