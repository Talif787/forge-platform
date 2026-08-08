# Applications read API (Phase 7)

The reconciler manages `Application` custom resources in Kubernetes. This endpoint
projects them into the control-plane REST API so the portal can show, for each
application, its declared intent and its live reconcile status.

Read-only by design: the portal observes what the operator manages; it never
creates Kubernetes objects directly.

## Endpoints

- `GET /api/v1/applications` (optional `?namespace=`) lists applications.
- `GET /api/v1/applications/{namespace}/{name}` gets one.

Each item carries the spec (image, port, desiredReplicas, tier, expose) and the
status the operator writes back (phase, readyReplicas, observedGeneration,
conditions).

## Cluster configuration

The control plane builds a Kubernetes client using standard config resolution
(in-cluster, then `KUBECONFIG`, then `~/.kube/config`). This is optional: if no
cluster is reachable at startup, the API logs a warning and these endpoints
respond `503 KUBERNETES_NOT_CONFIGURED`, while catalog and tenants continue to
work normally. So the API runs standalone without a cluster, and gains the
applications view automatically when one is configured.

## Testing

The disabled path (503) is covered by a plain unit test. The live read path is
covered by an envtest test that lists real `Application` resources from a running
API server:

```bash
make setup-envtest
KUBEBUILDER_ASSETS="$(setup-envtest use 1.31.0 -p path)" \
  go test -tags=envtest ./internal/modules/application/adapters/kube/...
```
