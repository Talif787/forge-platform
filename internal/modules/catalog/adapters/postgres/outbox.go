package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/forge-platform/forge/internal/modules/catalog/domain"
	"github.com/forge-platform/forge/internal/platform/id"
	platformpg "github.com/forge-platform/forge/internal/platform/postgres"
)

type Outbox struct{ db platformpg.DBTX }

type envelope struct {
	EventType   string `json:"eventType"`
	AggregateID string `json:"aggregateId"`
	Data        any    `json:"data"`
}

// Append writes domain events into the outbox in the same transaction as the
// state change, so events are never lost and never emitted for a rolled-back
// change. A relay publishes them to the event bus (Phase 2+).
func (o *Outbox) Append(ctx context.Context, events ...domain.Event) error {
	for _, e := range events {
		payload, err := json.Marshal(envelope{EventType: e.EventType(), AggregateID: e.AggregateID(), Data: e})
		if err != nil {
			return fmt.Errorf("marshal event %s: %w", e.EventType(), err)
		}
		if _, err := o.db.Exec(ctx, `
			INSERT INTO outbox_events (id, aggregate_id, event_type, payload, occurred_at)
			VALUES ($1, $2::uuid, $3, $4::jsonb, $5)`,
			id.New().String(), e.AggregateID(), e.EventType(), string(payload), e.OccurredAt()); err != nil {
			return fmt.Errorf("append outbox event: %w", err)
		}
	}
	return nil
}
