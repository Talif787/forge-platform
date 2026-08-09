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

	"github.com/forge-platform/forge/internal/modules/provisioning/app"
)

func router(reader app.Reader) *chi.Mux {
	h := NewHandlers(reader)
	r := chi.NewRouter()
	h.Mount(r)
	return r
}

func TestListDisabledReturns503(t *testing.T) {
	r := router(app.DisabledReader{})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/provisioning/workflows", nil))
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.Contains(t, rec.Body.String(), "TEMPORAL_NOT_CONFIGURED")
}

type fakeReader struct {
	views []app.WorkflowView
}

func (f fakeReader) List(context.Context) ([]app.WorkflowView, error) { return f.views, nil }
func (f fakeReader) Get(_ context.Context, id string) (app.WorkflowView, error) {
	for _, v := range f.views {
		if v.WorkflowID == id {
			return v, nil
		}
	}
	return app.WorkflowView{}, app.ErrNotFound
}

func TestListReturnsItems(t *testing.T) {
	reader := fakeReader{views: []app.WorkflowView{
		{WorkflowID: "provision-tenant-acme", RunID: "r1", Type: "ProvisionTenantWorkflow", Status: "Completed", HistoryLength: 21},
	}}
	r := router(reader)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/provisioning/workflows", nil))
	require.Equal(t, http.StatusOK, rec.Code)

	var body struct {
		Items []WorkflowResponse `json:"items"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Len(t, body.Items, 1)
	assert.Equal(t, "provision-tenant-acme", body.Items[0].WorkflowID)
	assert.Equal(t, "Completed", body.Items[0].Status)
}

func TestGetNotFound(t *testing.T) {
	r := router(fakeReader{})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/provisioning/workflows/missing", nil))
	assert.Equal(t, http.StatusNotFound, rec.Code)
}
