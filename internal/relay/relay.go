package relay

import (
	"context"
	"log/slog"
	"time"
)

// Record is one outbox row to be relayed.
type Record struct {
	ID          string
	AggregateID string
	EventType   string
	Payload     []byte
	OccurredAt  time.Time
}

// OutboxStore claims a batch of unpublished events, invokes publish for each,
// and marks them published, all atomically. Implementations must not mark an
// event published unless publish returned nil for it.
type OutboxStore interface {
	ClaimBatch(ctx context.Context, limit int, publish func(Record) error) (int, error)
}

// Publisher sends a message to the event bus. msgID enables server-side
// deduplication so a re-published event is delivered at most once per window.
type Publisher interface {
	Publish(ctx context.Context, subject, msgID string, data []byte) error
}

type Relay struct {
	store    OutboxStore
	pub      Publisher
	prefix   string
	batch    int
	interval time.Duration
	log      *slog.Logger
}

func New(store OutboxStore, pub Publisher, subjectPrefix string, batch int, interval time.Duration, log *slog.Logger) *Relay {
	if batch <= 0 {
		batch = 100
	}
	if interval <= 0 {
		interval = time.Second
	}
	return &Relay{store: store, pub: pub, prefix: subjectPrefix, batch: batch, interval: interval, log: log}
}

// DrainOnce publishes all currently pending events, looping until a batch comes
// back smaller than the limit (nothing left to claim).
func (r *Relay) DrainOnce(ctx context.Context) (int, error) {
	total := 0
	for {
		n, err := r.store.ClaimBatch(ctx, r.batch, func(rec Record) error {
			return r.pub.Publish(ctx, r.subjectFor(rec), rec.ID, rec.Payload)
		})
		total += n
		if err != nil {
			return total, err
		}
		if n < r.batch {
			return total, nil
		}
	}
}

func (r *Relay) subjectFor(rec Record) string {
	return r.prefix + "." + rec.EventType
}

// Run drains on a fixed interval until the context is cancelled. Errors are
// logged and retried on the next tick; unpublished events remain claimable.
func (r *Relay) Run(ctx context.Context) error {
	t := time.NewTicker(r.interval)
	defer t.Stop()
	r.log.Info("relay started", "interval", r.interval, "batch", r.batch)
	for {
		select {
		case <-ctx.Done():
			r.log.Info("relay stopping")
			return ctx.Err()
		case <-t.C:
			n, err := r.DrainOnce(ctx)
			if err != nil {
				r.log.Error("relay drain failed", "error", err.Error())
				continue
			}
			if n > 0 {
				r.log.Info("relay published events", "count", n)
			}
		}
	}
}
