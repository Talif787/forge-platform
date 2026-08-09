# Provisioning read API

Projects Temporal provisioning workflow executions into the control-plane REST
API, so the portal can show tenant-provisioning runs and their status.

Read-only: the portal observes workflow executions; provisioning is triggered out
of band by the worker and CLI.

## Endpoints

- `GET /api/v1/provisioning/workflows` lists provisioning workflow executions
  (workflowId, runId, type, status, startTime, closeTime, historyLength).
- `GET /api/v1/provisioning/workflows/{workflowId}` describes one execution.

## Temporal configuration

The control plane builds a lazy Temporal client from `FORGE_TEMPORAL_HOSTPORT`
and `FORGE_TEMPORAL_NAMESPACE`. It does not connect until first use, so a Temporal
outage never blocks startup. If Temporal is unreachable, these endpoints respond
`503 TEMPORAL_NOT_CONFIGURED` while catalog, tenants, and applications continue to
work.

## Testing

The disabled path (503) and list/get mapping are covered by unit tests with a
fake reader. The live path is exercised by running Temporal, the worker, and a
provisioning run, then listing workflows through the API.
