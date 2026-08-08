# Application reconciler (Phase 4)

This is the platform's centerpiece: the developer-facing `Application` custom
resource and the controller that turns it into hardened Kubernetes objects.

## The contract

A developer writes intent, not Kubernetes:

```yaml
apiVersion: platform.forge.dev/v1alpha1
kind: Application
metadata:
  name: payments-api
spec:
  image: ghcr.io/acme/payments:1.2.3
  port: 8080
  replicas: 3
  tier: 1
  expose: true
```

The `ApplicationSpec` contains no Kubernetes or cloud fields. The controller owns
the translation, so the platform can change how applications are deployed (base
image policy, sidecars, network shape, even the orchestrator) without any team
editing their manifest. That owned abstraction is the reason platform
standardization survives past year one.

## What the controller injects

For every Application, the reconciler produces a Deployment (and a Service when
`expose: true`) and adds the standards teams routinely forget:

- Resource requests and limits, defaulted from the criticality tier.
- Liveness and readiness probes on the declared port.
- A hardened security context (run as non-root, no privilege escalation,
  read-only root filesystem, all capabilities dropped).
- Standard labels and Prometheus scrape annotations.

Hardening is strict by default and opt-out per application. Omitting the
`security` block yields the full hardening above. An image that must write to
its filesystem can opt out of just the read-only root while keeping every other
control:

```yaml
spec:
  image: nginxinc/nginx-unprivileged:stable
  port: 8080
  security:
    readOnlyRootFilesystem: false
```

The Deployment and Service are owned by the Application (owner references), so
deleting the Application garbage-collects them. The controller reconciles
continuously: delete the Deployment by hand and it is recreated, because desired
state lives in the Application, not the cluster.

## Running against a local cluster (kind)

```bash
make kind-up          # create a local cluster on Cloud Shell's Docker
make install-crd      # register the Application CRD
make operator         # run the controller (foreground)
# in another tab:
make sample           # apply config/samples/application.yaml
kubectl get applications
kubectl get deployment,svc -l app.kubernetes.io/managed-by=forge
```

Delete the Deployment (`kubectl delete deploy hello`) and watch the controller
recreate it. Delete the Application and watch the Deployment and Service go with
it.

## Testing (envtest)

The controller tests run against a real kube-apiserver and etcd via envtest (no
kubelet, so pods do not actually run; the tests assert the objects the
controller creates). They are gated behind the `envtest` build tag.

```bash
make setup-envtest    # downloads the kube-apiserver/etcd test binaries once
make test-controller
```

## Reconcile idempotency

Owned objects are written with Kubernetes server-side apply (a stable field
owner), not read-modify-write. The operator authoritatively manages only the
fields it sets and leaves server-defaulted fields alone, so an unchanged
Application reconciles to a no-op (no resourceVersion churn, no self-triggered
requeues). The Application status is updated under a conflict retry and only when
it actually changes. A test asserts a second reconcile does not rewrite the
Deployment.
