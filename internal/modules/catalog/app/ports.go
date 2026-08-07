package app

import (
	"context"
	"time"

	"github.com/forge-platform/forge/internal/modules/catalog/domain"
)

// Repository is the persistence port for the Service aggregate. Writes enforce
// optimistic concurrency via the aggregate version.
type Repository interface {
	Insert(ctx context.Context, s *domain.Service) error
	Update(ctx context.Context, s *domain.Service, expectedVersion int64) error
	FindByID(ctx context.Context, id domain.ServiceID) (*domain.Service, error)
	FindByTenantAndName(ctx context.Context, tenant domain.TenantID, name domain.ServiceName) (*domain.Service, error)
	List(ctx context.Context, f ListFilter) ([]*domain.Service, string, error)
}

// Outbox persists domain events transactionally with the state change.
type Outbox interface {
	Append(ctx context.Context, events ...domain.Event) error
}

// Atomic runs a unit of work that spans the repository and the outbox in one
// transaction, guaranteeing the aggregate and its events commit together.
type Atomic interface {
	Execute(ctx context.Context, fn func(Repository, Outbox) error) error
}

// Reader provides non-transactional read access for queries.
type Reader interface {
	FindByID(ctx context.Context, id domain.ServiceID) (*domain.Service, error)
	List(ctx context.Context, f ListFilter) ([]*domain.Service, string, error)
}

type Clock interface{ Now() time.Time }

type SystemClock struct{}

func (SystemClock) Now() time.Time { return time.Now().UTC() }

type SortField string

const (
	SortCreatedAt SortField = "created_at"
	SortName      SortField = "name"
)

type SortOrder string

const (
	OrderAsc  SortOrder = "asc"
	OrderDesc SortOrder = "desc"
)

type ListFilter struct {
	TenantID  *domain.TenantID
	Lifecycle *domain.Lifecycle
	Tier      *domain.Tier
	Sort      SortField
	Order     SortOrder
	Limit     int
	Cursor    string
}
