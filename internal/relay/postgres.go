package relay

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PgStore claims outbox rows with FOR UPDATE SKIP LOCKED so multiple relay
// replicas never publish the same row, then publishes and marks them published
// inside one transaction. If publishing fails, the transaction rolls back and
// the rows stay claimable; duplicate publishes from a crash are absorbed by the
// stream's deduplication window.
type PgStore struct{ pool *pgxpool.Pool }

func NewPgStore(pool *pgxpool.Pool) *PgStore { return &PgStore{pool: pool} }

func (s *PgStore) ClaimBatch(ctx context.Context, limit int, publish func(Record) error) (int, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	rows, err := tx.Query(ctx, `
		SELECT id::text, aggregate_id::text, event_type, payload, occurred_at
		  FROM outbox_events
		 WHERE published_at IS NULL
		 ORDER BY created_at, id
		 LIMIT $1
		 FOR UPDATE SKIP LOCKED`, limit)
	if err != nil {
		return 0, fmt.Errorf("claim outbox: %w", err)
	}

	var recs []Record
	for rows.Next() {
		var r Record
		if err := rows.Scan(&r.ID, &r.AggregateID, &r.EventType, &r.Payload, &r.OccurredAt); err != nil {
			rows.Close()
			return 0, fmt.Errorf("scan outbox row: %w", err)
		}
		recs = append(recs, r)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	if len(recs) == 0 {
		return 0, nil
	}

	ids := make([]string, 0, len(recs))
	for _, r := range recs {
		if err := publish(r); err != nil {
			return 0, fmt.Errorf("publish %s: %w", r.ID, err)
		}
		ids = append(ids, r.ID)
	}

	if _, err := tx.Exec(ctx,
		`UPDATE outbox_events SET published_at = now() WHERE id = ANY($1::uuid[])`, ids); err != nil {
		return 0, fmt.Errorf("mark published: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit: %w", err)
	}
	return len(recs), nil
}
