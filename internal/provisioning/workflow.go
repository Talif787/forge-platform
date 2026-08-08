package provisioning

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// ProvisionTenantWorkflow provisions a tenant environment across several systems
// as a saga: each successful step registers a compensating action, and if a
// later step fails, the completed steps are rolled back in reverse order. This
// gives multi-system provisioning atomic-like semantics without a distributed
// transaction, and Temporal makes the whole thing durable and resumable across
// worker crashes.
func ProvisionTenantWorkflow(ctx workflow.Context, in ProvisionTenantInput) (ProvisionTenantResult, error) {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval: time.Second,
			MaximumAttempts: 3,
		},
	})
	logger := workflow.GetLogger(ctx)
	logger.Info("provisioning tenant", "slug", in.Slug)

	saga := newSaga()
	var a *Activities

	var ns NamespaceResult
	if err := workflow.ExecuteActivity(ctx, a.CreateNamespace, NamespaceRequest{Slug: in.Slug}).Get(ctx, &ns); err != nil {
		return ProvisionTenantResult{}, err
	}
	saga.addCompensation(func(ctx workflow.Context) error {
		return workflow.ExecuteActivity(ctx, a.DeleteNamespace, NamespaceRequest{Slug: in.Slug}).Get(ctx, nil)
	})

	if err := workflow.ExecuteActivity(ctx, a.ApplyQuota, QuotaRequest{Slug: in.Slug, MaxServices: in.MaxServices}).Get(ctx, nil); err != nil {
		return ProvisionTenantResult{}, saga.compensate(ctx, err)
	}
	saga.addCompensation(func(ctx workflow.Context) error {
		return workflow.ExecuteActivity(ctx, a.RemoveQuota, QuotaRequest{Slug: in.Slug}).Get(ctx, nil)
	})

	var dns DNSResult
	if err := workflow.ExecuteActivity(ctx, a.RegisterDNS, DNSRequest{Slug: in.Slug}).Get(ctx, &dns); err != nil {
		return ProvisionTenantResult{}, saga.compensate(ctx, err)
	}
	saga.addCompensation(func(ctx workflow.Context) error {
		return workflow.ExecuteActivity(ctx, a.DeregisterDNS, DNSRequest{Slug: in.Slug}).Get(ctx, nil)
	})

	if err := workflow.ExecuteActivity(ctx, a.CreateSecretPath, SecretRequest{Slug: in.Slug}).Get(ctx, nil); err != nil {
		return ProvisionTenantResult{}, saga.compensate(ctx, err)
	}
	saga.addCompensation(func(ctx workflow.Context) error {
		return workflow.ExecuteActivity(ctx, a.DeleteSecretPath, SecretRequest{Slug: in.Slug}).Get(ctx, nil)
	})

	if err := workflow.ExecuteActivity(ctx, a.Finalize, FinalizeRequest{Slug: in.Slug}).Get(ctx, nil); err != nil {
		return ProvisionTenantResult{}, saga.compensate(ctx, err)
	}

	logger.Info("tenant provisioned", "slug", in.Slug, "namespace", ns.Name, "endpoint", dns.FQDN)
	return ProvisionTenantResult{Namespace: ns.Name, Endpoint: dns.FQDN}, nil
}

// saga accumulates compensating actions and runs them in reverse on failure.
type saga struct {
	compensations []func(workflow.Context) error
}

func newSaga() *saga { return &saga{} }

func (s *saga) addCompensation(f func(workflow.Context) error) {
	s.compensations = append(s.compensations, f)
}

func (s *saga) compensate(ctx workflow.Context, cause error) error {
	logger := workflow.GetLogger(ctx)
	logger.Error("provisioning failed; rolling back", "cause", cause.Error())
	for i := len(s.compensations) - 1; i >= 0; i-- {
		if err := s.compensations[i](ctx); err != nil {
			// A failed compensation is logged but does not stop the rollback of
			// the remaining steps; the workflow still reports the original cause.
			logger.Error("compensation step failed", "error", err.Error())
		}
	}
	return fmt.Errorf("provisioning failed and was rolled back: %w", cause)
}
