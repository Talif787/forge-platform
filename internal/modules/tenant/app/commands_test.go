package app

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/forge-platform/forge/internal/modules/tenant/domain"
)

type fakeRepo struct {
	byID   map[string]*domain.Tenant
	bySlug map[string]*domain.Tenant
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{byID: map[string]*domain.Tenant{}, bySlug: map[string]*domain.Tenant{}}
}

func (f *fakeRepo) Insert(_ context.Context, t *domain.Tenant) error {
	if _, ok := f.bySlug[t.Slug().String()]; ok {
		return domain.ErrSlugConflict
	}
	f.byID[t.ID().String()] = t
	f.bySlug[t.Slug().String()] = t
	return nil
}

func (f *fakeRepo) Update(_ context.Context, t *domain.Tenant, expected int64) error {
	existing, ok := f.byID[t.ID().String()]
	if !ok {
		return domain.ErrTenantNotFound
	}
	if existing.Version() != expected {
		return domain.ErrVersionConflict
	}
	f.byID[t.ID().String()] = t
	return nil
}

func (f *fakeRepo) FindByID(_ context.Context, id domain.TenantID) (*domain.Tenant, error) {
	if t, ok := f.byID[id.String()]; ok {
		return cloneTenant(t), nil
	}
	return nil, domain.ErrTenantNotFound
}

func (f *fakeRepo) FindBySlug(_ context.Context, slug domain.Slug) (*domain.Tenant, error) {
	if t, ok := f.bySlug[slug.String()]; ok {
		return cloneTenant(t), nil
	}
	return nil, domain.ErrTenantNotFound
}

func (f *fakeRepo) List(context.Context, ListFilter) ([]*domain.Tenant, string, error) {
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

func cloneTenant(t *domain.Tenant) *domain.Tenant {
	return domain.Reconstitute(t.ID(), t.Name(), t.Slug(), t.Plan(), t.Status(), t.Quota(), t.Version(), t.CreatedAt(), t.UpdatedAt())
}

func newCommands() (*Commands, *fakeRepo, *fakeOutbox) {
	repo := newFakeRepo()
	outbox := &fakeOutbox{}
	return NewCommands(&fakeAtomic{repo: repo, outbox: outbox}, fixedClock{t: time.Now().UTC()}), repo, outbox
}

func validInput() CreateTenantInput {
	return CreateTenantInput{Name: "Acme Corp", Slug: "acme", Plan: "standard", MaxServices: 10}
}

func TestCreateTenant_Success(t *testing.T) {
	cmds, repo, outbox := newCommands()
	tn, err := cmds.CreateTenant(context.Background(), validInput())
	require.NoError(t, err)
	assert.Equal(t, domain.StatusActive, tn.Status())
	assert.Len(t, repo.byID, 1)
	require.Len(t, outbox.events, 1)
	assert.Equal(t, "tenant.created", outbox.events[0].EventType())
}

func TestCreateTenant_SlugConflict(t *testing.T) {
	cmds, _, _ := newCommands()
	_, err := cmds.CreateTenant(context.Background(), validInput())
	require.NoError(t, err)
	_, err = cmds.CreateTenant(context.Background(), validInput())
	require.ErrorIs(t, err, domain.ErrSlugConflict)
}

func TestChangeStatus_Success(t *testing.T) {
	cmds, _, _ := newCommands()
	tn, err := cmds.CreateTenant(context.Background(), validInput())
	require.NoError(t, err)
	updated, err := cmds.ChangeStatus(context.Background(), ChangeStatusInput{
		TenantID: tn.ID().String(), Status: "suspended",
	})
	require.NoError(t, err)
	assert.Equal(t, domain.StatusSuspended, updated.Status())
	assert.Equal(t, int64(2), updated.Version())
}

func TestUpdateQuota_VersionConflict(t *testing.T) {
	cmds, _, _ := newCommands()
	tn, err := cmds.CreateTenant(context.Background(), validInput())
	require.NoError(t, err)
	_, err = cmds.UpdateQuota(context.Background(), UpdateQuotaInput{
		TenantID: tn.ID().String(), MaxServices: 20, ExpectedVersion: 99,
	})
	require.ErrorIs(t, err, domain.ErrVersionConflict)
}
