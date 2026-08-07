package app

import (
	"context"
	"time"

	"github.com/forge-platform/forge/internal/modules/tenant/domain"
)

type Repository interface {
	Insert(ctx context.Context, t *domain.Tenant) error
	Update(ctx context.Context, t *domain.Tenant, expectedVersion int64) error
	FindByID(ctx context.Context, id domain.TenantID) (*domain.Tenant, error)
	FindBySlug(ctx context.Context, slug domain.Slug) (*domain.Tenant, error)
	List(ctx context.Context, f ListFilter) ([]*domain.Tenant, string, error)
}

type Outbox interface {
	Append(ctx context.Context, events ...domain.Event) error
}

type Atomic interface {
	Execute(ctx context.Context, fn func(Repository, Outbox) error) error
}

type Reader interface {
	FindByID(ctx context.Context, id domain.TenantID) (*domain.Tenant, error)
	List(ctx context.Context, f ListFilter) ([]*domain.Tenant, string, error)
}

type Clock interface{ Now() time.Time }

type SystemClock struct{}

func (SystemClock) Now() time.Time { return time.Now().UTC() }

type SortOrder string

const (
	OrderAsc  SortOrder = "asc"
	OrderDesc SortOrder = "desc"
)

type ListFilter struct {
	Status *domain.Status
	Plan   *domain.Plan
	Order  SortOrder
	Limit  int
	Cursor string
}
