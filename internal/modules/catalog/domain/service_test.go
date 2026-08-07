package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mustService(t *testing.T, onCall string) *Service {
	t.Helper()
	tenant, err := TenantIDFromString(uuid.NewString())
	require.NoError(t, err)
	name, err := NewServiceName("payments-api")
	require.NoError(t, err)
	tier, err := NewTier(1)
	require.NoError(t, err)
	own, err := NewOwnership("payments", onCall)
	require.NoError(t, err)
	return Register(NewServiceID(), tenant, name, "handles payments", tier, "https://git/payments", own, time.Now().UTC())
}

func TestRegister_StartsExperimentalAndEmitsEvent(t *testing.T) {
	s := mustService(t, "pd://payments")
	assert.Equal(t, LifecycleExperimental, s.Lifecycle())
	assert.Equal(t, int64(1), s.Version())
	events := s.PullEvents()
	require.Len(t, events, 1)
	assert.Equal(t, "catalog.service.registered", events[0].EventType())
	assert.Empty(t, s.PullEvents(), "events should be cleared after pull")
}

func TestChangeLifecycle_ToProductionRequiresOnCall(t *testing.T) {
	s := mustService(t, "")
	_ = s.PullEvents()

	err := s.ChangeLifecycle(LifecycleProduction, time.Now().UTC())
	require.ErrorIs(t, err, ErrOnCallRequired)
	assert.Equal(t, LifecycleExperimental, s.Lifecycle())

	require.NoError(t, s.ChangeOwnership(Ownership{OwningTeam: "payments", OnCallRef: "pd://payments"}, time.Now().UTC()))
	require.NoError(t, s.ChangeLifecycle(LifecycleProduction, time.Now().UTC()))
	assert.Equal(t, LifecycleProduction, s.Lifecycle())
}

func TestChangeLifecycle_IllegalTransitionRejected(t *testing.T) {
	s := mustService(t, "pd://payments")
	_ = s.PullEvents()
	require.NoError(t, s.Retire(time.Now().UTC()))

	err := s.ChangeLifecycle(LifecycleProduction, time.Now().UTC())
	require.ErrorIs(t, err, ErrIllegalTransition)
}

func TestChangeOwnership_RejectedAfterRetire(t *testing.T) {
	s := mustService(t, "pd://payments")
	require.NoError(t, s.Retire(time.Now().UTC()))

	err := s.ChangeOwnership(Ownership{OwningTeam: "new-team", OnCallRef: "pd://new"}, time.Now().UTC())
	require.ErrorIs(t, err, ErrAlreadyRetired)
}

func TestVersionIncrementsOnMutation(t *testing.T) {
	s := mustService(t, "pd://payments")
	require.NoError(t, s.ChangeLifecycle(LifecycleProduction, time.Now().UTC()))
	assert.Equal(t, int64(2), s.Version())
	require.NoError(t, s.ChangeLifecycle(LifecycleDeprecated, time.Now().UTC()))
	assert.Equal(t, int64(3), s.Version())
}

func TestValueObjectValidation(t *testing.T) {
	_, err := NewServiceName("Invalid_Name")
	require.ErrorIs(t, err, ErrInvalidServiceName)
	_, err = NewTier(5)
	require.ErrorIs(t, err, ErrInvalidTier)
	_, err = ParseLifecycle("unknown")
	require.ErrorIs(t, err, ErrInvalidLifecycle)
	_, err = NewOwnership("", "")
	require.ErrorIs(t, err, ErrInvalidOwnership)
}
