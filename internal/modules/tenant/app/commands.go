package app

import (
	"context"

	"github.com/forge-platform/forge/internal/modules/tenant/domain"
)

type Commands struct {
	atomic Atomic
	clock  Clock
}

func NewCommands(atomic Atomic, clock Clock) *Commands {
	return &Commands{atomic: atomic, clock: clock}
}

type CreateTenantInput struct {
	Name        string
	Slug        string
	Plan        string
	MaxServices int
}

func (c *Commands) CreateTenant(ctx context.Context, in CreateTenantInput) (*domain.Tenant, error) {
	name, err := domain.NewTenantName(in.Name)
	if err != nil {
		return nil, err
	}
	slug, err := domain.NewSlug(in.Slug)
	if err != nil {
		return nil, err
	}
	plan, err := domain.ParsePlan(in.Plan)
	if err != nil {
		return nil, err
	}
	quota, err := domain.NewQuota(in.MaxServices)
	if err != nil {
		return nil, err
	}

	tenant := domain.Create(domain.NewTenantID(), name, slug, plan, quota, c.clock.Now())

	err = c.atomic.Execute(ctx, func(repo Repository, outbox Outbox) error {
		if _, err := repo.FindBySlug(ctx, slug); err == nil {
			return domain.ErrSlugConflict
		} else if err != domain.ErrTenantNotFound {
			return err
		}
		if err := repo.Insert(ctx, tenant); err != nil {
			return err
		}
		return outbox.Append(ctx, tenant.PullEvents()...)
	})
	if err != nil {
		return nil, err
	}
	return tenant, nil
}

type ChangeStatusInput struct {
	TenantID        string
	Status          string
	ExpectedVersion int64
}

func (c *Commands) ChangeStatus(ctx context.Context, in ChangeStatusInput) (*domain.Tenant, error) {
	id, err := domain.TenantIDFromString(in.TenantID)
	if err != nil {
		return nil, err
	}
	status, err := domain.ParseStatus(in.Status)
	if err != nil {
		return nil, err
	}
	return c.mutate(ctx, id, in.ExpectedVersion, func(t *domain.Tenant) error {
		return t.ChangeStatus(status, c.clock.Now())
	})
}

type UpdateQuotaInput struct {
	TenantID        string
	MaxServices     int
	ExpectedVersion int64
}

func (c *Commands) UpdateQuota(ctx context.Context, in UpdateQuotaInput) (*domain.Tenant, error) {
	id, err := domain.TenantIDFromString(in.TenantID)
	if err != nil {
		return nil, err
	}
	quota, err := domain.NewQuota(in.MaxServices)
	if err != nil {
		return nil, err
	}
	return c.mutate(ctx, id, in.ExpectedVersion, func(t *domain.Tenant) error {
		return t.UpdateQuota(quota, c.clock.Now())
	})
}

func (c *Commands) mutate(ctx context.Context, id domain.TenantID, expectedVersion int64, apply func(*domain.Tenant) error) (*domain.Tenant, error) {
	var result *domain.Tenant
	err := c.atomic.Execute(ctx, func(repo Repository, outbox Outbox) error {
		tenant, err := repo.FindByID(ctx, id)
		if err != nil {
			return err
		}
		current := tenant.Version()
		if err := apply(tenant); err != nil {
			return err
		}
		if err := repo.Update(ctx, tenant, expectedVersionOr(expectedVersion, current)); err != nil {
			return err
		}
		if err := outbox.Append(ctx, tenant.PullEvents()...); err != nil {
			return err
		}
		result = tenant
		return nil
	})
	return result, err
}

func expectedVersionOr(provided, current int64) int64 {
	if provided > 0 {
		return provided
	}
	return current
}
