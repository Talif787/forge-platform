package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/forge-platform/forge/internal/modules/catalog/app"
	"github.com/forge-platform/forge/internal/modules/catalog/domain"
)

// Store implements app.Atomic (transactional writes) and app.Reader
// (non-transactional reads) over a single pool.
type Store struct{ pool *pgxpool.Pool }

func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

func (s *Store) Execute(ctx context.Context, fn func(app.Repository, app.Outbox) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	repo := &Repository{db: tx}
	outbox := &Outbox{db: tx}
	if err := fn(repo, outbox); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

func (s *Store) FindByID(ctx context.Context, id domain.ServiceID) (*domain.Service, error) {
	return (&Repository{db: s.pool}).FindByID(ctx, id)
}

func (s *Store) List(ctx context.Context, f app.ListFilter) ([]*domain.Service, string, error) {
	return (&Repository{db: s.pool}).List(ctx, f)
}
