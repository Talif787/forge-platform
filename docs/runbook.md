# Operational runbook (Phase 1)

## Health

- Liveness: `GET /healthz` (process only; never checks dependencies).
- Readiness: `GET /readyz` (verifies database connectivity; failing removes the
  instance from load balancing).
- Metrics: `GET /metrics` (Prometheus).

## Signals and alerts

- `forge_http_requests_total{status="Internal Server Error"}` rising: inspect
  logs by `correlation_id`; every 5xx logs the correlating id and error code.
- Readiness failing: check database reachability and pool saturation.
- 429 responses rising: a client is exceeding its rate class; identify by
  subject in logs.

## Common incidents

- **Database unavailable.** Readiness fails and the instance is pulled from
  rotation automatically. Writes fail cleanly with a 500 and a correlation id.
  Recovery is automatic on database return; no data loss for committed
  transactions.
- **Version conflict (409 VERSION_CONFLICT).** Expected under concurrent edits.
  Clients should refetch, reapply, and retry with the current ETag.
- **Migration failure at startup.** The process exits non-zero before binding.
  Inspect the failing migration named in the error; fix forward with a new
  migration rather than editing an applied one.

## Correlation

Every request carries a correlation id (generated or propagated via
`X-Correlation-Id`), present on every log line and returned in the response
header. When tracing is enabled, logs also carry the trace id for pivoting into
the trace.
