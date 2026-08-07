# Configuration

All configuration is environment-driven (Twelve-Factor). Defaults suit local
development. Production requires `FORGE_AUTH_MODE=jwks`; `hmac` is rejected.

| Variable | Default | Purpose |
|---|---|---|
| FORGE_ENV | development | Environment name; `production` tightens defaults |
| FORGE_HTTP_ADDR | :8080 | Listen address |
| FORGE_SHUTDOWN_TIMEOUT | 15s | Graceful shutdown budget |
| FORGE_DB_URL | (required) | Postgres connection string |
| FORGE_DB_MAX_CONNS | 10 | Pool maximum connections |
| FORGE_DB_MIN_CONNS | 2 | Pool minimum connections |
| FORGE_DB_MAX_CONN_LIFETIME | 1h | Connection max lifetime |
| FORGE_DB_MAX_CONN_IDLE_TIME | 30m | Connection max idle time |
| FORGE_AUTH_MODE | hmac | `hmac` (dev) or `jwks` (OIDC) |
| FORGE_AUTH_HMAC_SECRET | (dev) | HS256 secret; required when mode=hmac |
| FORGE_AUTH_JWKS_URL | | JWKS endpoint; required when mode=jwks |
| FORGE_AUTH_ISSUER | | Expected token issuer (jwks) |
| FORGE_AUTH_AUDIENCE | | Expected token audience (jwks) |
| FORGE_OTEL_SERVICE_NAME | forge-control-plane | Service name in telemetry |
| FORGE_OTEL_OTLP_ENDPOINT | | OTLP HTTP endpoint; empty disables export |
| FORGE_RATE_LIMIT_RPS | 50 | Per-identity sustained rate |
| FORGE_RATE_LIMIT_BURST | 100 | Per-identity burst |

Configuration is validated at startup; invalid combinations fail fast with a
descriptive message before the server binds.
