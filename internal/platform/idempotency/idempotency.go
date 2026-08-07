package idempotency

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/forge-platform/forge/internal/platform/id"
)

type Record struct {
	Status int
	Body   []byte
}

// Store persists idempotent responses so a retried request with the same
// Idempotency-Key returns the original outcome without re-executing the effect.
type Store struct{ pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

func (s *Store) Lookup(ctx context.Context, key string) (*Record, bool, error) {
	var rec Record
	err := s.pool.QueryRow(ctx,
		`SELECT response_status, response_body FROM idempotency_keys WHERE idem_key=$1`, key).
		Scan(&rec.Status, &rec.Body)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("lookup idempotency key: %w", err)
	}
	return &rec, true, nil
}

// Save stores the response. A concurrent insert for the same key is resolved by
// ON CONFLICT DO NOTHING so the first writer wins.
func (s *Store) Save(ctx context.Context, key, requestHash string, status int, body []byte) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO idempotency_keys (id, idem_key, request_hash, response_status, response_body)
		VALUES ($1, $2, $3, $4, $5::jsonb)
		ON CONFLICT (idem_key) DO NOTHING`,
		id.New().String(), key, requestHash, status, string(body))
	if err != nil {
		return fmt.Errorf("save idempotency key: %w", err)
	}
	return nil
}
