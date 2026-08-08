package provisioning

import "context"

// Activities wrap the Provisioner so Temporal can invoke each step as a durable,
// retryable unit. They are deliberately thin; the interesting logic (ordering
// and compensation) lives in the workflow.
type Activities struct {
	p Provisioner
}

func NewActivities(p Provisioner) *Activities { return &Activities{p: p} }

func (a *Activities) CreateNamespace(ctx context.Context, in NamespaceRequest) (NamespaceResult, error) {
	name, err := a.p.CreateNamespace(ctx, in.Slug)
	return NamespaceResult{Name: name}, err
}

func (a *Activities) DeleteNamespace(ctx context.Context, in NamespaceRequest) error {
	return a.p.DeleteNamespace(ctx, in.Slug)
}

func (a *Activities) ApplyQuota(ctx context.Context, in QuotaRequest) error {
	return a.p.ApplyQuota(ctx, in.Slug, in.MaxServices)
}

func (a *Activities) RemoveQuota(ctx context.Context, in QuotaRequest) error {
	return a.p.RemoveQuota(ctx, in.Slug)
}

func (a *Activities) RegisterDNS(ctx context.Context, in DNSRequest) (DNSResult, error) {
	fqdn, err := a.p.RegisterDNS(ctx, in.Slug)
	return DNSResult{FQDN: fqdn}, err
}

func (a *Activities) DeregisterDNS(ctx context.Context, in DNSRequest) error {
	return a.p.DeregisterDNS(ctx, in.Slug)
}

func (a *Activities) CreateSecretPath(ctx context.Context, in SecretRequest) error {
	return a.p.CreateSecretPath(ctx, in.Slug)
}

func (a *Activities) DeleteSecretPath(ctx context.Context, in SecretRequest) error {
	return a.p.DeleteSecretPath(ctx, in.Slug)
}

func (a *Activities) Finalize(ctx context.Context, in FinalizeRequest) error {
	return a.p.Finalize(ctx, in.Slug)
}
