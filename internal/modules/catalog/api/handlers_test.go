package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/forge-platform/forge/internal/modules/catalog/app"
	"github.com/forge-platform/forge/internal/modules/catalog/domain"
)

// memStore implements app.Repository, app.Atomic, and app.Reader in memory.
type memStore struct {
	byID   map[string]*domain.Service
	byName map[string]*domain.Service
}

func newMemStore() *memStore {
	return &memStore{byID: map[string]*domain.Service{}, byName: map[string]*domain.Service{}}
}

func key(t domain.TenantID, n domain.ServiceName) string { return t.String() + "/" + n.String() }

func (m *memStore) Insert(_ context.Context, s *domain.Service) error {
	if _, ok := m.byName[key(s.TenantID(), s.Name())]; ok {
		return domain.ErrNameConflict
	}
	m.byID[s.ID().String()] = s
	m.byName[key(s.TenantID(), s.Name())] = s
	return nil
}

func (m *memStore) Update(_ context.Context, s *domain.Service, expected int64) error {
	existing, ok := m.byID[s.ID().String()]
	if !ok {
		return domain.ErrServiceNotFound
	}
	if existing.Version() != expected {
		return domain.ErrVersionConflict
	}
	m.byID[s.ID().String()] = s
	return nil
}

func (m *memStore) FindByID(_ context.Context, id domain.ServiceID) (*domain.Service, error) {
	if s, ok := m.byID[id.String()]; ok {
		return cloneService(s), nil
	}
	return nil, domain.ErrServiceNotFound
}

func (m *memStore) FindByTenantAndName(_ context.Context, t domain.TenantID, n domain.ServiceName) (*domain.Service, error) {
	if s, ok := m.byName[key(t, n)]; ok {
		return cloneService(s), nil
	}
	return nil, domain.ErrServiceNotFound
}

func (m *memStore) List(context.Context, app.ListFilter) ([]*domain.Service, string, error) {
	out := make([]*domain.Service, 0, len(m.byID))
	for _, s := range m.byID {
		out = append(out, s)
	}
	return out, "", nil
}

type noopOutbox struct{}

func (noopOutbox) Append(context.Context, ...domain.Event) error { return nil }

func (m *memStore) Execute(ctx context.Context, fn func(app.Repository, app.Outbox) error) error {
	return fn(m, noopOutbox{})
}

func newRouter() (*chi.Mux, *memStore) {
	store := newMemStore()
	h := NewHandlers(app.NewCommands(store, app.SystemClock{}), app.NewQueries(store), nil)
	r := chi.NewRouter()
	h.Mount(r)
	return r, store
}

func TestRegisterAndGet(t *testing.T) {
	r, _ := newRouter()
	body := `{"tenantId":"` + uuid.NewString() + `","name":"payments-api","tier":1,"owningTeam":"payments","onCallRef":"pd://payments"}`

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/services", strings.NewReader(body)))
	require.Equal(t, http.StatusCreated, rec.Code)

	var created ServiceResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))
	assert.Equal(t, "payments-api", created.Name)
	assert.Equal(t, "experimental", created.Lifecycle)
	assert.NotEmpty(t, rec.Header().Get("Location"))
	assert.Equal(t, "1", rec.Header().Get("ETag"))

	getRec := httptest.NewRecorder()
	r.ServeHTTP(getRec, httptest.NewRequest(http.MethodGet, "/services/"+created.ID, nil))
	require.Equal(t, http.StatusOK, getRec.Code)
}

func TestRegisterValidationError(t *testing.T) {
	r, _ := newRouter()
	body := `{"tenantId":"` + uuid.NewString() + `","name":"payments-api","tier":9,"owningTeam":"payments"}`
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/services", strings.NewReader(body)))
	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	assert.Contains(t, rec.Body.String(), "INVALID_TIER")
}

func TestGetNotFound(t *testing.T) {
	r, _ := newRouter()
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/services/"+uuid.NewString(), nil))
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestChangeLifecycleFlow(t *testing.T) {
	r, _ := newRouter()
	body := `{"tenantId":"` + uuid.NewString() + `","name":"orders-api","tier":2,"owningTeam":"orders","onCallRef":"pd://orders"}`
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/services", strings.NewReader(body)))
	require.Equal(t, http.StatusCreated, rec.Code)
	var created ServiceResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))

	patch := httptest.NewRequest(http.MethodPatch, "/services/"+created.ID+"/lifecycle", strings.NewReader(`{"lifecycle":"production"}`))
	patch.Header.Set("If-Match", "1")
	patchRec := httptest.NewRecorder()
	r.ServeHTTP(patchRec, patch)
	require.Equal(t, http.StatusOK, patchRec.Code)

	var updated ServiceResponse
	require.NoError(t, json.Unmarshal(patchRec.Body.Bytes(), &updated))
	assert.Equal(t, "production", updated.Lifecycle)
	assert.Equal(t, int64(2), updated.Version)
}

// cloneService returns an independent copy so mutating a loaded aggregate does
// not mutate the stored one, mirroring how a real database returns snapshots.
func cloneService(s *domain.Service) *domain.Service {
	return domain.Reconstitute(s.ID(), s.TenantID(), s.Name(), s.Description(), s.Tier(), s.Lifecycle(), s.Repository(), s.Ownership(), s.Version(), s.CreatedAt(), s.UpdatedAt())
}
