package events

import (
	"context"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// Connect opens a NATS connection with infinite reconnect and returns a
// JetStream context.
func Connect(name, url string) (*nats.Conn, jetstream.JetStream, error) {
	nc, err := nats.Connect(url,
		nats.Name(name),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(2*time.Second),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("connect nats: %w", err)
	}
	js, err := jetstream.New(nc)
	if err != nil {
		nc.Close()
		return nil, nil, fmt.Errorf("create jetstream: %w", err)
	}
	return nc, js, nil
}

// EnsureStream creates or updates the events stream. The duplicate window makes
// publishing idempotent: a message re-published with the same ID within the
// window is deduplicated by the server, which is what makes at-least-once relay
// delivery safe.
func EnsureStream(ctx context.Context, js jetstream.JetStream, name, subjectPrefix string) error {
	_, err := js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:        name,
		Subjects:    []string{subjectPrefix + ".>"},
		Storage:     jetstream.FileStorage,
		Retention:   jetstream.LimitsPolicy,
		Duplicates:  2 * time.Minute,
		MaxAge:      30 * 24 * time.Hour,
		Description: "Forge domain events relayed from the transactional outbox",
	})
	if err != nil {
		return fmt.Errorf("ensure stream %q: %w", name, err)
	}
	return nil
}

// JetStreamPublisher publishes messages with a deduplication ID.
type JetStreamPublisher struct{ js jetstream.JetStream }

func NewJetStreamPublisher(js jetstream.JetStream) *JetStreamPublisher {
	return &JetStreamPublisher{js: js}
}

func (p *JetStreamPublisher) Publish(ctx context.Context, subject, msgID string, data []byte) error {
	if _, err := p.js.Publish(ctx, subject, data, jetstream.WithMsgID(msgID)); err != nil {
		return fmt.Errorf("publish %s: %w", subject, err)
	}
	return nil
}
