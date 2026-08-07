package relay_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/forge-platform/forge/internal/relay"
)

// fakeStore models the outbox: it hands unpublished records to publish and only
// marks them published if publish succeeded for the whole batch, mirroring the
// transactional Postgres store.
type fakeStore struct {
	recs      []relay.Record
	published map[string]bool
}

func newFakeStore(recs ...relay.Record) *fakeStore {
	return &fakeStore{recs: recs, published: map[string]bool{}}
}

func (f *fakeStore) ClaimBatch(_ context.Context, limit int, publish func(relay.Record) error) (int, error) {
	var batch []relay.Record
	for _, r := range f.recs {
		if !f.published[r.ID] {
			batch = append(batch, r)
			if len(batch) == limit {
				break
			}
		}
	}
	if len(batch) == 0 {
		return 0, nil
	}
	for _, r := range batch {
		if err := publish(r); err != nil {
			return 0, err // rollback: nothing marked published
		}
	}
	for _, r := range batch {
		f.published[r.ID] = true
	}
	return len(batch), nil
}

type fakePublisher struct {
	subjects []string
	msgIDs   []string
	failOn   string
}

func (p *fakePublisher) Publish(_ context.Context, subject, msgID string, _ []byte) error {
	if msgID == p.failOn {
		return errors.New("publish failed")
	}
	p.subjects = append(p.subjects, subject)
	p.msgIDs = append(p.msgIDs, msgID)
	return nil
}

func rec(id, eventType string) relay.Record {
	return relay.Record{ID: id, AggregateID: "agg-" + id, EventType: eventType, Payload: []byte(`{}`)}
}

func TestDrainPublishesAllPendingWithCorrectSubjects(t *testing.T) {
	store := newFakeStore(
		rec("1", "catalog.service.registered"),
		rec("2", "catalog.service.lifecycle_changed"),
	)
	pub := &fakePublisher{}
	r := relay.New(store, pub, "forge.events", 10, time.Second, discardLogger())

	n, err := r.DrainOnce(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 2, n)
	assert.Equal(t, []string{
		"forge.events.catalog.service.registered",
		"forge.events.catalog.service.lifecycle_changed",
	}, pub.subjects)
	assert.Equal(t, []string{"1", "2"}, pub.msgIDs, "message id must be the outbox row id for dedup")
}

func TestDrainLoopsUntilDrained(t *testing.T) {
	store := newFakeStore(rec("1", "e"), rec("2", "e"), rec("3", "e"))
	pub := &fakePublisher{}
	// batch size 2 forces two claim iterations (2 then 1)
	r := relay.New(store, pub, "forge.events", 2, time.Second, discardLogger())

	n, err := r.DrainOnce(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 3, n)
	assert.Len(t, pub.subjects, 3)
}

func TestPublishFailureLeavesEventsClaimable(t *testing.T) {
	store := newFakeStore(rec("1", "e"), rec("2", "e"), rec("3", "e"))
	pub := &fakePublisher{failOn: "2"}
	r := relay.New(store, pub, "forge.events", 10, time.Second, discardLogger())

	_, err := r.DrainOnce(context.Background())
	require.Error(t, err)
	assert.False(t, store.published["1"], "batch must roll back so nothing is marked published")

	// After the transient failure clears, a later drain publishes everything.
	pub.failOn = ""
	n, err := r.DrainOnce(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 3, n)
}
