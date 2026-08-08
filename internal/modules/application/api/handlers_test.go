package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/forge-platform/forge/internal/modules/application/app"
)

func router(reader app.Reader) *chi.Mux {
	h := NewHandlers(reader)
	r := chi.NewRouter()
	h.Mount(r)
	return r
}

// The disabled reader (no cluster configured) must surface as 503, not a crash,
// so the rest of the control plane keeps serving.
func TestListDisabledReturns503(t *testing.T) {
	r := router(app.DisabledReader{})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/applications", nil))
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.Contains(t, rec.Body.String(), "KUBERNETES_NOT_CONFIGURED")
}

func TestGetDisabledReturns503(t *testing.T) {
	r := router(app.DisabledReader{})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/applications/default/hello", nil))
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
}

type fakeReader struct {
	views []app.ApplicationView
	err   error
}

func (f fakeReader) List(_ context.Context, _ string) ([]app.ApplicationView, error) {
	return f.views, f.err
}

func (f fakeReader) Get(_ context.Context, _, name string) (app.ApplicationView, error) {
	for _, v := range f.views {
		if v.Name == name {
			return v, nil
		}
	}
	return app.ApplicationView{}, app.ErrNotFound
}

func TestListReturnsItems(t *testing.T) {
	reader := fakeReader{views: []app.ApplicationView{
		{Namespace: "default", Name: "hello", Image: "nginx:1.27", Port: 8080, DesiredReplicas: 2, Tier: 3, Phase: "Ready", ReadyReplicas: 2},
	}}
	r := router(reader)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/applications", nil))
	require.Equal(t, http.StatusOK, rec.Code)

	var body struct {
		Items []ApplicationResponse `json:"items"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Len(t, body.Items, 1)
	assert.Equal(t, "hello", body.Items[0].Name)
	assert.Equal(t, "Ready", body.Items[0].Phase)
}

func TestGetNotFound(t *testing.T) {
	r := router(fakeReader{})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/applications/default/missing", nil))
	assert.Equal(t, http.StatusNotFound, rec.Code)
}
