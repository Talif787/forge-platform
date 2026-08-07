//go:build integration

package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/forge-platform/forge/internal/modules/catalog/app"
	"github.com/forge-platform/forge/internal/modules/catalog/domain"
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
		testcontainers.WithWaitStrategy(wait.ForLog("database system is ready to accept connections").WithOccurrence(2).WithStartupTimeout(60*time.Second)),
	)
	require.NoError(t, err)

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	require.NoError(t, platformpg.Migrate(ctx, pool, migrations.FS))

	return pool, func() {
		pool.Close()
		_ = container.Terminate(ctx)
	}
}

func newService(t *testing.T, name string, created time.Time) *domain.Service {
	t.Helper()
	tenant, err := domain.TenantIDFromString(uuid.NewString())
	require.NoError(t, err)
	sname, err := domain.NewServiceName(name)
	require.NoError(t, err)
	tier, err := domain.NewTier(2)
	require.NoError(t, err)
	own, err := domain.NewOwnership("platform", "pd://platform")
	require.NoError(t, err)
	return domain.Register(domain.NewServiceID(), tenant, sname, "", tier, "", own, created)
}

func insert(t *testing.T, store *Store, s *domain.Service) {
	t.Helper()
	err := store.Execute(context.Background(), func(repo app.Repository, outbox app.Outbox) error {
		if err := repo.Insert(context.Background(), s); err != nil {
			return err
		}
		return outbox.Append(context.Background(), s.PullEvents()...)
	})
	require.NoError(t, err)
}

func TestInsertAndFindByID(t *testing.T) {
	pool, teardown := setup(t)
	defer teardown()
	store := NewStore(pool)

	svc := newService(t, "payments-api", time.Now().UTC())
	insert(t, store, svc)

	found, err := store.FindByID(context.Background(), svc.ID())
	require.NoError(t, err)
	assert.Equal(t, svc.Name().String(), found.Name().String())
	assert.Equal(t, domain.LifecycleExperimental, found.Lifecycle())
}

func TestOptimisticConcurrency(t *testing.T) {
	pool, teardown := setup(t)
	defer teardown()
	store := NewStore(pool)

	svc := newService(t, "orders-api", time.Now().UTC())
	insert(t, store, svc)

	require.NoError(t, svc.ChangeLifecycle(domain.LifecycleProduction, time.Now().UTC()))
	err := store.Execute(context.Background(), func(repo app.Repository, _ app.Outbox) error {
		return repo.Update(context.Background(), svc, 999)
	})
	require.ErrorIs(t, err, domain.ErrVersionConflict)
}

func TestKeysetPagination(t *testing.T) {
	pool, teardown := setup(t)
	defer teardown()
	store := NewStore(pool)

	base := time.Now().UTC()
	for i := 0; i < 3; i++ {
		insert(t, store, newService(t, "svc-"+uuid.NewString()[:8], base.Add(time.Duration(i)*time.Second)))
	}

	first, next, err := store.List(context.Background(), app.ListFilter{Sort: app.SortCreatedAt, Order: app.OrderDesc, Limit: 2})
	require.NoError(t, err)
	require.Len(t, first, 2)
	require.NotEmpty(t, next)

	second, _, err := store.List(context.Background(), app.ListFilter{Sort: app.SortCreatedAt, Order: app.OrderDesc, Limit: 2, Cursor: next})
	require.NoError(t, err)
	require.Len(t, second, 1)
}
