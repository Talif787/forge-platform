package app

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/forge-platform/forge/internal/modules/catalog/domain"
)

type fakeRepo struct {
	byID   map[string]*domain.Service
	byName map[string]*domain.Service
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{byID: map[string]*domain.Service{}, byName: map[string]*domain.Service{}}
}

func nameKey(t domain.TenantID, n domain.ServiceName) string { return t.String() + "/" + n.String() }

func (f *fakeRepo) Insert(_ context.Context, s *domain.Service) error {
	if _, ok := f.byName[nameKey(s.TenantID(), s.Name())]; ok {
		return domain.ErrNameConflict
	}
	f.byID[s.ID().String()] = s
	f.byName[nameKey(s.TenantID(), s.Name())] = s
	return nil
}

func (f *fakeRepo) Update(_ context.Context, s *domain.Service, expectedVersion int64) error {
	existing, ok := f.byID[s.ID().String()]
	if !ok {
		return domain.ErrServiceNotFound
	}
	if existing.Version() != expectedVersion {
		return domain.ErrVersionConflict
	}
	f.byID[s.ID().String()] = s
	return nil
}

func (f *fakeRepo) FindByID(_ context.Context, id domain.ServiceID) (*domain.Service, error) {
	if s, ok := f.byID[id.String()]; ok {
		return s, nil
	}
	return nil, domain.ErrServiceNotFound
}

func (f *fakeRepo) FindByTenantAndName(_ context.Context, t domain.TenantID, n domain.ServiceName) (*domain.Service, error) {
	if s, ok := f.byName[nameKey(t, n)]; ok {
		return s, nil
	}
	return nil, domain.ErrServiceNotFound
}

func (f *fakeRepo) List(context.Context, ListFilter) ([]*domain.Service, string, error) {
	return nil, "", nil
}

type fakeOutbox struct{ events []domain.Event }

func (o *fakeOutbox) Append(_ context.Context, e ...domain.Event) error {
	o.events = append(o.events, e...)
	return nil
}

type fakeAtomic struct {
	repo   *fakeRepo
	outbox *fakeOutbox
}

func (a *fakeAtomic) Execute(_ context.Context, fn func(Repository, Outbox) error) error {
	return fn(a.repo, a.outbox)
}

type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time { return c.t }

func newCommands() (*Commands, *fakeRepo, *fakeOutbox) {
	repo := newFakeRepo()
	outbox := &fakeOutbox{}
	atomic := &fakeAtomic{repo: repo, outbox: outbox}
	return NewCommands(atomic, fixedClock{t: time.Now().UTC()}), repo, outbox
}

func validRegisterInput() RegisterServiceInput {
	return RegisterServiceInput{
		TenantID:   uuid.NewString(),
		Name:       "payments-api",
		Tier:       1,
		OwningTeam: "payments",
		OnCallRef:  "pd://payments",
	}
}

func TestRegisterService_Success(t *testing.T) {
	cmds, repo, outbox := newCommands()
	svc, err := cmds.RegisterService(context.Background(), validRegisterInput())
	require.NoError(t, err)
	assert.Equal(t, domain.LifecycleExperimental, svc.Lifecycle())
	assert.Len(t, repo.byID, 1)
	require.Len(t, outbox.events, 1)
	assert.Equal(t, "catalog.service.registered", outbox.events[0].EventType())
}

func TestRegisterService_DuplicateNameConflict(t *testing.T) {
	cmds, _, _ := newCommands()
	in := validRegisterInput()
	_, err := cmds.RegisterService(context.Background(), in)
	require.NoError(t, err)
	_, err = cmds.RegisterService(context.Background(), in)
	require.ErrorIs(t, err, domain.ErrNameConflict)
}

func TestRegisterService_InvalidTierRejected(t *testing.T) {
	cmds, _, _ := newCommands()
	in := validRegisterInput()
	in.Tier = 9
	_, err := cmds.RegisterService(context.Background(), in)
	require.ErrorIs(t, err, domain.ErrInvalidTier)
}

func TestChangeLifecycle_ToProductionWithoutOnCallRejected(t *testing.T) {
	cmds, _, _ := newCommands()
	in := validRegisterInput()
	in.OnCallRef = ""
	svc, err := cmds.RegisterService(context.Background(), in)
	require.NoError(t, err)

	_, err = cmds.ChangeLifecycle(context.Background(), ChangeLifecycleInput{
		ServiceID: svc.ID().String(), Lifecycle: "production",
	})
	require.ErrorIs(t, err, domain.ErrOnCallRequired)
}

func TestChangeOwnership_VersionConflict(t *testing.T) {
	cmds, _, _ := newCommands()
	svc, err := cmds.RegisterService(context.Background(), validRegisterInput())
	require.NoError(t, err)

	_, err = cmds.ChangeOwnership(context.Background(), ChangeOwnershipInput{
		ServiceID: svc.ID().String(), OwningTeam: "new-team", OnCallRef: "pd://new",
		ExpectedVersion: 99,
	})
	require.ErrorIs(t, err, domain.ErrVersionConflict)
}
