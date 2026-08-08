package provisioning

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
)

// Provisioner abstracts the external systems a tenant environment spans. The
// simulated implementation makes the workflow runnable and testable without real
// cloud, DNS, or secret-store credentials; a production implementation would
// call Kubernetes, Route 53, and Vault. Every method is idempotent so Temporal
// can safely retry activities.
type Provisioner interface {
	CreateNamespace(ctx context.Context, slug string) (string, error)
	DeleteNamespace(ctx context.Context, slug string) error
	ApplyQuota(ctx context.Context, slug string, maxServices int) error
	RemoveQuota(ctx context.Context, slug string) error
	RegisterDNS(ctx context.Context, slug string) (string, error)
	DeregisterDNS(ctx context.Context, slug string) error
	CreateSecretPath(ctx context.Context, slug string) error
	DeleteSecretPath(ctx context.Context, slug string) error
	Finalize(ctx context.Context, slug string) error
}

const dnsZone = "forge.example.com"

// SimulatedProvisioner is an in-memory Provisioner for local runs and tests.
type SimulatedProvisioner struct {
	log *slog.Logger

	mu         sync.Mutex
	namespaces map[string]bool
	quotas     map[string]int
	dns        map[string]bool
	secrets    map[string]bool

	// Test hooks: force a step to fail to exercise compensation.
	FailRegisterDNS bool
}

func NewSimulatedProvisioner(log *slog.Logger) *SimulatedProvisioner {
	return &SimulatedProvisioner{
		log:        log,
		namespaces: map[string]bool{},
		quotas:     map[string]int{},
		dns:        map[string]bool{},
		secrets:    map[string]bool{},
	}
}

func (p *SimulatedProvisioner) CreateNamespace(_ context.Context, slug string) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	name := "tenant-" + slug
	p.namespaces[slug] = true
	p.log.Info("namespace created", "slug", slug, "name", name)
	return name, nil
}

func (p *SimulatedProvisioner) DeleteNamespace(_ context.Context, slug string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.namespaces, slug)
	p.log.Info("namespace deleted (compensation)", "slug", slug)
	return nil
}

func (p *SimulatedProvisioner) ApplyQuota(_ context.Context, slug string, maxServices int) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.quotas[slug] = maxServices
	p.log.Info("quota applied", "slug", slug, "maxServices", maxServices)
	return nil
}

func (p *SimulatedProvisioner) RemoveQuota(_ context.Context, slug string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.quotas, slug)
	p.log.Info("quota removed (compensation)", "slug", slug)
	return nil
}

func (p *SimulatedProvisioner) RegisterDNS(_ context.Context, slug string) (string, error) {
	if p.FailRegisterDNS {
		return "", fmt.Errorf("dns provider unavailable for %q", slug)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.dns[slug] = true
	fqdn := slug + "." + dnsZone
	p.log.Info("dns registered", "slug", slug, "fqdn", fqdn)
	return fqdn, nil
}

func (p *SimulatedProvisioner) DeregisterDNS(_ context.Context, slug string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.dns, slug)
	p.log.Info("dns deregistered (compensation)", "slug", slug)
	return nil
}

func (p *SimulatedProvisioner) CreateSecretPath(_ context.Context, slug string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.secrets[slug] = true
	p.log.Info("secret path created", "slug", slug, "path", "vault://"+slug)
	return nil
}

func (p *SimulatedProvisioner) DeleteSecretPath(_ context.Context, slug string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.secrets, slug)
	p.log.Info("secret path deleted (compensation)", "slug", slug)
	return nil
}

func (p *SimulatedProvisioner) Finalize(_ context.Context, slug string) error {
	p.log.Info("tenant provisioning finalized", "slug", slug)
	return nil
}

// Inspection helpers for tests.
func (p *SimulatedProvisioner) HasNamespace(slug string) bool { return p.has(p.namespaces, slug) }
func (p *SimulatedProvisioner) HasDNS(slug string) bool       { return p.has(p.dns, slug) }
func (p *SimulatedProvisioner) HasSecret(slug string) bool    { return p.has(p.secrets, slug) }

func (p *SimulatedProvisioner) HasQuota(slug string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	_, ok := p.quotas[slug]
	return ok
}

func (p *SimulatedProvisioner) has(m map[string]bool, slug string) bool {
	if m == nil {
		return false
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return m[slug]
}
