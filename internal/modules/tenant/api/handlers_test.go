package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/forge-platform/forge/internal/modules/tenant/app"
	"github.com/forge-platform/forge/internal/modules/tenant/domain"
)

type memStore struct {
	byID   map[string]*domain.Tenant
	bySlug map[string]*domain.Tenant
}

func newMemStore() *memStore {
	return &memStore{byID: map[string]*domain.Tenant{}, bySlug: map[string]*domain.Tenant{}}
}

func clone(t *domain.Tenant) *domain.Tenant {
	return domain.Reconstitute(t.ID(), t.Name(), t.Slug(), t.Plan(), t.Status(), t.Quota(), t.Version(), t.CreatedAt(), t.UpdatedAt())
}

func (m *memStore) Insert(_ context.Context, t *domain.Tenant) error {
	if _, ok := m.bySlug[t.Slug().String()]; ok {
		return domain.ErrSlugConflict
	}
	m.byID[t.ID().String()] = t
	m.bySlug[t.Slug().String()] = t
	return nil
}

func (m *memStore) Update(_ context.Context, t *domain.Tenant, expected int64) error {
	existing, ok := m.byID[t.ID().String()]
	if !ok {
		return domain.ErrTenantNotFound
	}
	if existing.Version() != expected {
		return domain.ErrVersionConflict
	}
	m.byID[t.ID().String()] = t
	return nil
}

func (m *memStore) FindByID(_ context.Context, id domain.TenantID) (*domain.Tenant, error) {
	if t, ok := m.byID[id.String()]; ok {
		return clone(t), nil
	}
	return nil, domain.ErrTenantNotFound
}

func (m *memStore) FindBySlug(_ context.Context, slug domain.Slug) (*domain.Tenant, error) {
	if t, ok := m.bySlug[slug.String()]; ok {
		return clone(t), nil
	}
	return nil, domain.ErrTenantNotFound
}

func (m *memStore) List(context.Context, app.ListFilter) ([]*domain.Tenant, string, error) {
	out := make([]*domain.Tenant, 0, len(m.byID))
	for _, t := range m.byID {
		out = append(out, clone(t))
	}
	return out, "", nil
}

type noopOutbox struct{}

func (noopOutbox) Append(context.Context, ...domain.Event) error { return nil }

func (m *memStore) Execute(ctx context.Context, fn func(app.Repository, app.Outbox) error) error {
	return fn(m, noopOutbox{})
}

func newRouter() *chi.Mux {
	store := newMemStore()
	h := NewHandlers(app.NewCommands(store, app.SystemClock{}), app.NewQueries(store))
	r := chi.NewRouter()
	h.Mount(r)
	return r
}

func TestCreateAndGet(t *testing.T) {
	r := newRouter()
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/tenants",
		strings.NewReader(`{"name":"Acme Corp","slug":"acme","plan":"standard","maxServices":10}`)))
	require.Equal(t, http.StatusCreated, rec.Code)

	var created TenantResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))
	assert.Equal(t, "acme", created.Slug)
	assert.Equal(t, "active", created.Status)
	assert.Equal(t, "1", rec.Header().Get("ETag"))

	getRec := httptest.NewRecorder()
	r.ServeHTTP(getRec, httptest.NewRequest(http.MethodGet, "/tenants/"+created.ID, nil))
	require.Equal(t, http.StatusOK, getRec.Code)
}

func TestCreateValidationError(t *testing.T) {
	r := newRouter()
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/tenants",
		strings.NewReader(`{"name":"Acme","slug":"acme","plan":"platinum","maxServices":10}`)))
	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	assert.Contains(t, rec.Body.String(), "INVALID_PLAN")
}

func TestStatusFlow(t *testing.T) {
	r := newRouter()
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/tenants",
		strings.NewReader(`{"name":"Beta","slug":"beta","plan":"free","maxServices":5}`)))
	require.Equal(t, http.StatusCreated, rec.Code)
	var created TenantResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))

	patch := httptest.NewRequest(http.MethodPatch, "/tenants/"+created.ID+"/status", strings.NewReader(`{"status":"suspended"}`))
	patch.Header.Set("If-Match", "1")
	patchRec := httptest.NewRecorder()
	r.ServeHTTP(patchRec, patch)
	require.Equal(t, http.StatusOK, patchRec.Code)

	var updated TenantResponse
	require.NoError(t, json.Unmarshal(patchRec.Body.Bytes(), &updated))
	assert.Equal(t, "suspended", updated.Status)
	assert.Equal(t, int64(2), updated.Version)
}
