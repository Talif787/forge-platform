# Policy and observability (Phase 6)

Two guardrail layers plus automatic observability for every managed service.

## Admission policy

### CEL on the Application (enforced by the API server)

The Application CRD carries validation rules in CEL, so the API server rejects a
non-conforming resource at admission with no webhook or certificates:

- the image must carry an explicit tag that is not `:latest`
- a tier 1 (most critical) application must run at least 2 replicas

Try it:

```bash
kubectl apply -f - <<'YAML'
apiVersion: platform.forge.dev/v1alpha1
kind: Application
metadata: {name: bad, namespace: default}
spec: {image: nginx:latest, port: 80}
YAML
# rejected: image must specify an explicit tag other than ':latest'
```

### Kyverno (cluster-wide guardrail)

CEL protects the Application contract; Kyverno protects the whole cluster,
catching workloads created outside the platform path. The policies in
`config/policy/` require resource limits, forbid the `:latest` tag, and require
non-root containers on every Pod.

```bash
make kyverno-install     # installs Kyverno into the cluster
make install-policy      # applies the ClusterPolicies
# now a raw Pod that violates a policy is rejected:
kubectl run bad --image=nginx:latest   # blocked by disallow-latest-tag
```

Forge-managed workloads already satisfy these because the controller injects
limits, tags, and the security context; Kyverno is defense in depth.

## Observability injection

For every Application, the controller generates an observability ConfigMap
(`<app>-observability`) containing a Grafana dashboard and Prometheus alert
rules, owned by the Application so it is garbage-collected with it. Combined with
the Prometheus scrape annotations the controller already sets on pods, every
managed service gets metrics discovery, a dashboard, and alerts automatically,
with no work from the developer.

- The dashboard ConfigMap is labeled `grafana_dashboard: "1"`, which the Grafana
  sidecar auto-loads.
- The alert rules cover a high 5xx error rate and insufficient ready replicas.

```bash
kubectl get configmap hello-observability -o yaml
```

The ConfigMap delivery uses only core resources, so it works without the
Prometheus Operator. Where that operator is present, the same generation would
emit `ServiceMonitor` and `PrometheusRule` custom resources instead.
