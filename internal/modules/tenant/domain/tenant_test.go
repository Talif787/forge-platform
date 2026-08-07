package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mustTenant(t *testing.T) *Tenant {
	t.Helper()
	name, err := NewTenantName("Acme Corp")
	require.NoError(t, err)
	slug, err := NewSlug("acme")
	require.NoError(t, err)
	quota, err := NewQuota(10)
	require.NoError(t, err)
	return Create(NewTenantID(), name, slug, PlanStandard, quota, time.Now().UTC())
}

func TestCreate_ActiveAndEmitsEvent(t *testing.T) {
	tn := mustTenant(t)
	assert.Equal(t, StatusActive, tn.Status())
	assert.Equal(t, int64(1), tn.Version())
	events := tn.PullEvents()
	require.Len(t, events, 1)
	assert.Equal(t, "tenant.created", events[0].EventType())
}

func TestChangeStatus_Transitions(t *testing.T) {
	tn := mustTenant(t)
	_ = tn.PullEvents()
	require.NoError(t, tn.ChangeStatus(StatusSuspended, time.Now().UTC()))
	assert.Equal(t, StatusSuspended, tn.Status())
	assert.Equal(t, int64(2), tn.Version())
	require.NoError(t, tn.ChangeStatus(StatusActive, time.Now().UTC()))
	assert.Equal(t, StatusActive, tn.Status())
}

func TestChangeStatus_IllegalFromArchived(t *testing.T) {
	tn := mustTenant(t)
	require.NoError(t, tn.ChangeStatus(StatusArchived, time.Now().UTC()))
	err := tn.ChangeStatus(StatusActive, time.Now().UTC())
	require.ErrorIs(t, err, ErrIllegalTransition)
}

func TestUpdateQuota_RejectedWhenArchived(t *testing.T) {
	tn := mustTenant(t)
	require.NoError(t, tn.ChangeStatus(StatusArchived, time.Now().UTC()))
	q, _ := NewQuota(50)
	err := tn.UpdateQuota(q, time.Now().UTC())
	require.ErrorIs(t, err, ErrArchived)
}

func TestValueValidation(t *testing.T) {
	_, err := NewSlug("Not_A_Slug")
	require.ErrorIs(t, err, ErrInvalidSlug)
	_, err = ParsePlan("platinum")
	require.ErrorIs(t, err, ErrInvalidPlan)
	_, err = ParseStatus("frozen")
	require.ErrorIs(t, err, ErrInvalidStatus)
	_, err = NewQuota(-1)
	require.ErrorIs(t, err, ErrInvalidQuota)
}
