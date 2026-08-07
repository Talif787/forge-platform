package app

import (
	"context"

	"github.com/forge-platform/forge/internal/modules/catalog/domain"
)

type Commands struct {
	atomic Atomic
	clock  Clock
}

func NewCommands(atomic Atomic, clock Clock) *Commands {
	return &Commands{atomic: atomic, clock: clock}
}

type RegisterServiceInput struct {
	TenantID    string
	Name        string
	Description string
	Tier        int
	Repository  string
	OwningTeam  string
	OnCallRef   string
}

func (c *Commands) RegisterService(ctx context.Context, in RegisterServiceInput) (*domain.Service, error) {
	tenantID, err := domain.TenantIDFromString(in.TenantID)
	if err != nil {
		return nil, err
	}
	name, err := domain.NewServiceName(in.Name)
	if err != nil {
		return nil, err
	}
	tier, err := domain.NewTier(in.Tier)
	if err != nil {
		return nil, err
	}
	ownership, err := domain.NewOwnership(in.OwningTeam, in.OnCallRef)
	if err != nil {
		return nil, err
	}

	svc := domain.Register(domain.NewServiceID(), tenantID, name, in.Description, tier, in.Repository, ownership, c.clock.Now())

	err = c.atomic.Execute(ctx, func(repo Repository, outbox Outbox) error {
		if _, err := repo.FindByTenantAndName(ctx, tenantID, name); err == nil {
			return domain.ErrNameConflict
		} else if !isNotFound(err) {
			return err
		}
		if err := repo.Insert(ctx, svc); err != nil {
			return err
		}
		return outbox.Append(ctx, svc.PullEvents()...)
	})
	if err != nil {
		return nil, err
	}
	return svc, nil
}

type ChangeOwnershipInput struct {
	ServiceID       string
	OwningTeam      string
	OnCallRef       string
	ExpectedVersion int64
}

func (c *Commands) ChangeOwnership(ctx context.Context, in ChangeOwnershipInput) (*domain.Service, error) {
	id, err := domain.ServiceIDFromString(in.ServiceID)
	if err != nil {
		return nil, err
	}
	ownership, err := domain.NewOwnership(in.OwningTeam, in.OnCallRef)
	if err != nil {
		return nil, err
	}
	return c.mutate(ctx, id, in.ExpectedVersion, func(s *domain.Service) error {
		return s.ChangeOwnership(ownership, c.clock.Now())
	})
}

type ChangeLifecycleInput struct {
	ServiceID       string
	Lifecycle       string
	ExpectedVersion int64
}

func (c *Commands) ChangeLifecycle(ctx context.Context, in ChangeLifecycleInput) (*domain.Service, error) {
	id, err := domain.ServiceIDFromString(in.ServiceID)
	if err != nil {
		return nil, err
	}
	lifecycle, err := domain.ParseLifecycle(in.Lifecycle)
	if err != nil {
		return nil, err
	}
	return c.mutate(ctx, id, in.ExpectedVersion, func(s *domain.Service) error {
		return s.ChangeLifecycle(lifecycle, c.clock.Now())
	})
}

type RetireServiceInput struct {
	ServiceID       string
	ExpectedVersion int64
}

func (c *Commands) RetireService(ctx context.Context, in RetireServiceInput) (*domain.Service, error) {
	id, err := domain.ServiceIDFromString(in.ServiceID)
	if err != nil {
		return nil, err
	}
	return c.mutate(ctx, id, in.ExpectedVersion, func(s *domain.Service) error {
		return s.Retire(c.clock.Now())
	})
}

func (c *Commands) mutate(ctx context.Context, id domain.ServiceID, expectedVersion int64, apply func(*domain.Service) error) (*domain.Service, error) {
	var result *domain.Service
	err := c.atomic.Execute(ctx, func(repo Repository, outbox Outbox) error {
		svc, err := repo.FindByID(ctx, id)
		if err != nil {
			return err
		}
		current := svc.Version()
		if err := apply(svc); err != nil {
			return err
		}
		if err := repo.Update(ctx, svc, expectedVersionOr(expectedVersion, current)); err != nil {
			return err
		}
		if err := outbox.Append(ctx, svc.PullEvents()...); err != nil {
			return err
		}
		result = svc
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

func isNotFound(err error) bool {
	return err == domain.ErrServiceNotFound
}
