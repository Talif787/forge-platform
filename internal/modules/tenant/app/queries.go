package app

import (
	"context"

	"github.com/forge-platform/forge/internal/modules/tenant/domain"
)

type Queries struct {
	reader Reader
}

func NewQueries(reader Reader) *Queries { return &Queries{reader: reader} }

func (q *Queries) GetTenant(ctx context.Context, tenantID string) (*domain.Tenant, error) {
	id, err := domain.TenantIDFromString(tenantID)
	if err != nil {
		return nil, err
	}
	return q.reader.FindByID(ctx, id)
}

type ListResult struct {
	Tenants    []*domain.Tenant
	NextCursor string
}

func (q *Queries) ListTenants(ctx context.Context, f ListFilter) (ListResult, error) {
	if f.Limit <= 0 || f.Limit > 200 {
		f.Limit = 50
	}
	if f.Order == "" {
		f.Order = OrderDesc
	}
	tenants, next, err := q.reader.List(ctx, f)
	if err != nil {
		return ListResult{}, err
	}
	return ListResult{Tenants: tenants, NextCursor: next}, nil
}
