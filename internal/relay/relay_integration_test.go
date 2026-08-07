//go:build integration

package relay_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/forge-platform/forge/internal/platform/events"
	platformpg "github.com/forge-platform/forge/internal/platform/postgres"
	"github.com/forge-platform/forge/internal/relay"
	"github.com/forge-platform/forge/migrations"
)

func startPostgres(t *testing.T, ctx context.Context) (*pgxpool.Pool, func()) {
	t.Helper()
	c, err := tcpostgres.Run(ctx, "postgres:16-alpine",
		tcpostgres.WithDatabase("forge"),
		tcpostgres.WithUsername("forge"),
		tcpostgres.WithPassword("forge"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).WithStartupTimeout(60*time.Second)),
	)
	require.NoError(t, err)
	dsn, err := c.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	require.NoError(t, platformpg.Migrate(ctx, pool, migrations.FS))
	return pool, func() { pool.Close(); _ = c.Terminate(ctx) }
}

func startNATS(t *testing.T, ctx context.Context) (string, func()) {
	t.Helper()
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "nats:2.10-alpine",
			Cmd:          []string{"-js"},
			ExposedPorts: []string{"4222/tcp"},
			WaitingFor:   wait.ForLog("Server is ready").WithStartupTimeout(30 * time.Second),
		},
		Started: true,
	})
	require.NoError(t, err)
	host, err := c.Host(ctx)
	require.NoError(t, err)
	port, err := c.MappedPort(ctx, "4222/tcp")
	require.NoError(t, err)
	return fmt.Sprintf("nats://%s:%s", host, port.Port()), func() { _ = c.Terminate(ctx) }
}

func insertOutbox(t *testing.T, ctx context.Context, pool *pgxpool.Pool, eventType string) string {
	t.Helper()
	id := uuid.Must(uuid.NewV7()).String()
	agg := uuid.Must(uuid.NewV7()).String()
	_, err := pool.Exec(ctx, `
		INSERT INTO outbox_events (id, aggregate_id, event_type, payload, occurred_at)
		VALUES ($1::uuid, $2::uuid, $3, $4::jsonb, now())`,
		id, agg, eventType, fmt.Sprintf(`{"eventType":%q,"aggregateId":%q}`, eventType, agg))
	require.NoError(t, err)
	return id
}

func TestRelayPublishesOutboxToJetStream(t *testing.T) {
	ctx := context.Background()

	pool, stopPG := startPostgres(t, ctx)
	defer stopPG()
	natsURL, stopNATS := startNATS(t, ctx)
	defer stopNATS()

	insertOutbox(t, ctx, pool, "catalog.service.registered")
	insertOutbox(t, ctx, pool, "catalog.service.lifecycle_changed")

	nc, js, err := events.Connect("test-relay", natsURL)
	require.NoError(t, err)
	defer nc.Drain()
	require.NoError(t, events.EnsureStream(ctx, js, "FORGE_EVENTS", "forge.events"))

	// A consumer must exist before publishing to capture the messages.
	cons, err := js.CreateOrUpdateConsumer(ctx, "FORGE_EVENTS", jetstream.ConsumerConfig{
		Durable:       "test-consumer",
		AckPolicy:     jetstream.AckExplicitPolicy,
		FilterSubject: "forge.events.>",
	})
	require.NoError(t, err)

	r := relay.New(relay.NewPgStore(pool), events.NewJetStreamPublisher(js), "forge.events", 100, time.Second, discardLogger())
	n, err := r.DrainOnce(ctx)
	require.NoError(t, err)
	assert.Equal(t, 2, n)

	// All rows should now be marked published.
	var unpublished int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM outbox_events WHERE published_at IS NULL`).Scan(&unpublished))
	assert.Equal(t, 0, unpublished)

	// And the two messages should be retrievable from the stream.
	batch, err := cons.Fetch(2, jetstream.FetchMaxWait(5*time.Second))
	require.NoError(t, err)
	var subjects []string
	for msg := range batch.Messages() {
		subjects = append(subjects, msg.Subject())
		require.NoError(t, msg.Ack())
	}
	require.NoError(t, batch.Error())
	assert.ElementsMatch(t, []string{
		"forge.events.catalog.service.registered",
		"forge.events.catalog.service.lifecycle_changed",
	}, subjects)
}

func TestRelayIsIdempotentOnRepublish(t *testing.T) {
	ctx := context.Background()
	pool, stopPG := startPostgres(t, ctx)
	defer stopPG()
	natsURL, stopNATS := startNATS(t, ctx)
	defer stopNATS()

	id := insertOutbox(t, ctx, pool, "catalog.service.registered")

	nc, js, err := events.Connect("test-relay", natsURL)
	require.NoError(t, err)
	defer nc.Drain()
	require.NoError(t, events.EnsureStream(ctx, js, "FORGE_EVENTS", "forge.events"))

	pub := events.NewJetStreamPublisher(js)
	// Publish the same message id twice directly; the dedup window must collapse
	// it to a single stored message.
	require.NoError(t, pub.Publish(ctx, "forge.events.catalog.service.registered", id, []byte(`{}`)))
	require.NoError(t, pub.Publish(ctx, "forge.events.catalog.service.registered", id, []byte(`{}`)))

	stream, err := js.Stream(ctx, "FORGE_EVENTS")
	require.NoError(t, err)
	info, err := stream.Info(ctx)
	require.NoError(t, err)
	assert.Equal(t, uint64(1), info.State.Msgs, "duplicate message id must be deduplicated")
}
