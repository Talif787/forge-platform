package app

import (
	"context"

	"github.com/forge-platform/forge/internal/modules/catalog/domain"
)

type Queries struct {
	reader Reader
}

func NewQueries(reader Reader) *Queries { return &Queries{reader: reader} }

func (q *Queries) GetService(ctx context.Context, serviceID string) (*domain.Service, error) {
	id, err := domain.ServiceIDFromString(serviceID)
	if err != nil {
		return nil, err
	}
	return q.reader.FindByID(ctx, id)
}

type ListResult struct {
	Services   []*domain.Service
	NextCursor string
}

func (q *Queries) ListServices(ctx context.Context, f ListFilter) (ListResult, error) {
	if f.Limit <= 0 || f.Limit > 200 {
		f.Limit = 50
	}
	if f.Sort == "" {
		f.Sort = SortCreatedAt
	}
	if f.Order == "" {
		f.Order = OrderDesc
	}
	services, next, err := q.reader.List(ctx, f)
	if err != nil {
		return ListResult{}, err
	}
	return ListResult{Services: services, NextCursor: next}, nil
}
