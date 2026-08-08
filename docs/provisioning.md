# Provisioning workflows (Phase 5)

Provisioning a tenant environment touches several systems (a namespace, a
resource quota, DNS, a secret store) with no distributed transaction across
them. A crash partway through must not leave orphaned resources. This phase
implements that as a durable Temporal workflow using the saga pattern.

## The saga

`ProvisionTenantWorkflow` runs the steps in order, and each successful step
registers a compensating action:

| Step | Compensation |
|---|---|
| Create namespace | Delete namespace |
| Apply resource quota | Remove quota |
| Register DNS | Deregister DNS |
| Create secret path | Delete secret path |
| Finalize | (none) |

If any step fails, the workflow runs the registered compensations in reverse
order and returns the original error, so the tenant environment is either fully
provisioned or fully rolled back. Temporal makes the workflow durable: activities
are retried per a retry policy, and the workflow resumes exactly where it left
off if a worker crashes.

The external systems are behind a `Provisioner` interface. The included
`SimulatedProvisioner` makes the workflow runnable and testable with no cloud,
DNS, or Vault credentials; a production implementation would call Kubernetes,
Route 53, and Vault. Every activity is idempotent so retries are safe.

## Running locally

The tests need no server (they use Temporal's in-memory test environment). For a
live run, start a local Temporal dev server, the worker, and a provisioning
request:

```bash
# install the temporal CLI once: https://docs.temporal.io/cli
make temporal-dev          # tab 1: local server + web UI at http://localhost:8233
make worker                # tab 2: hosts the workflow and activities
make provision SLUG=acme MAX=10   # tab 3: starts a provisioning workflow
```

Watch the workflow in the Temporal Web UI. To see compensation live, point the
worker at a provisioner configured to fail a step, or exercise it through the
tests below.

## Tests

```bash
go test -race -count=1 ./internal/provisioning/...
```

`TestProvisionTenant_Success` asserts every resource ends up provisioned.
`TestProvisionTenant_CompensatesOnFailure` forces the DNS step to fail and
asserts the namespace and quota created before it were rolled back.

## Integration point

In the full system, the tenant.created event (already published to NATS by the
Phase 2 relay) would trigger this workflow through an event consumer. It is a CLI
here to keep the phase self-contained and the trigger explicit.
