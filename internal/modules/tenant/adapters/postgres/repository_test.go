//go:build integration

package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/forge-platform/forge/internal/modules/tenant/app"
	"github.com/forge-platform/forge/internal/modules/tenant/domain"
	platformpg "github.com/forge-platform/forge/internal/platform/postgres"
	"github.com/forge-platform/forge/migrations"
)

func setup(t *testing.T) (*pgxpool.Pool, func()) {
	t.Helper()
	ctx := context.Background()
	container, err := tcpostgres.Run(ctx, "postgres:16-alpine",
		tcpostgres.WithDatabase("forge"),
		tcpostgres.WithUsername("forge"),
		tcpostgres.WithPassword("forge"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).WithStartupTimeout(60*time.Second)),
	)
	require.NoError(t, err)
	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	require.NoError(t, platformpg.Migrate(ctx, pool, migrations.FS))
	return pool, func() { pool.Close(); _ = container.Terminate(ctx) }
}

func newTenant(t *testing.T, slug string, created time.Time) *domain.Tenant {
	t.Helper()
	name, err := domain.NewTenantName("Test " + slug)
	require.NoError(t, err)
	s, err := domain.NewSlug(slug)
	require.NoError(t, err)
	q, err := domain.NewQuota(10)
	require.NoError(t, err)
	return domain.Create(domain.NewTenantID(), name, s, domain.PlanStandard, q, created)
}

func insert(t *testing.T, store *Store, tn *domain.Tenant) {
	t.Helper()
	err := store.Execute(context.Background(), func(repo app.Repository, outbox app.Outbox) error {
		if err := repo.Insert(context.Background(), tn); err != nil {
			return err
		}
		return outbox.Append(context.Background(), tn.PullEvents()...)
	})
	require.NoError(t, err)
}

func TestInsertFindAndOutbox(t *testing.T) {
	pool, teardown := setup(t)
	defer teardown()
	store := NewStore(pool)

	tn := newTenant(t, "acme", time.Now().UTC())
	insert(t, store, tn)

	found, err := store.FindByID(context.Background(), tn.ID())
	require.NoError(t, err)
	assert.Equal(t, "acme", found.Slug().String())
	assert.Equal(t, domain.StatusActive, found.Status())

	// tenant.created must have landed in the shared outbox for the relay.
	var n int
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT count(*) FROM outbox_events WHERE event_type='tenant.created'`).Scan(&n))
	assert.Equal(t, 1, n)
}

func TestOptimisticConcurrency(t *testing.T) {
	pool, teardown := setup(t)
	defer teardown()
	store := NewStore(pool)

	tn := newTenant(t, "beta", time.Now().UTC())
	insert(t, store, tn)

	require.NoError(t, tn.ChangeStatus(domain.StatusSuspended, time.Now().UTC()))
	err := store.Execute(context.Background(), func(repo app.Repository, _ app.Outbox) error {
		return repo.Update(context.Background(), tn, 999)
	})
	require.ErrorIs(t, err, domain.ErrVersionConflict)
}

func TestSlugConflict(t *testing.T) {
	pool, teardown := setup(t)
	defer teardown()
	store := NewStore(pool)

	insert(t, store, newTenant(t, "dup", time.Now().UTC()))
	err := store.Execute(context.Background(), func(repo app.Repository, _ app.Outbox) error {
		return repo.Insert(context.Background(), newTenant(t, "dup", time.Now().UTC()))
	})
	require.ErrorIs(t, err, domain.ErrSlugConflict)
}
