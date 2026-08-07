package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/forge-platform/forge/internal/modules/tenant/app"
	"github.com/forge-platform/forge/internal/modules/tenant/domain"
	"github.com/forge-platform/forge/internal/platform/apperr"
	"github.com/forge-platform/forge/internal/platform/httpx"
)

type Handlers struct {
	commands *app.Commands
	queries  *app.Queries
}

func NewHandlers(commands *app.Commands, queries *app.Queries) *Handlers {
	return &Handlers{commands: commands, queries: queries}
}

func (h *Handlers) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateTenantRequest
	if !httpx.Decode(w, r, &req) {
		return
	}
	tenant, err := h.commands.CreateTenant(r.Context(), app.CreateTenantInput{
		Name: req.Name, Slug: req.Slug, Plan: req.Plan, MaxServices: req.MaxServices,
	})
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	w.Header().Set("Location", "/api/v1/tenants/"+tenant.ID().String())
	setETag(w, tenant.Version())
	httpx.JSON(w, http.StatusCreated, toResponse(tenant))
}

func (h *Handlers) Get(w http.ResponseWriter, r *http.Request) {
	tenant, err := h.queries.GetTenant(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	setETag(w, tenant.Version())
	httpx.JSON(w, http.StatusOK, toResponse(tenant))
}

func (h *Handlers) List(w http.ResponseWriter, r *http.Request) {
	filter, err := parseListFilter(r)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	result, err := h.queries.ListTenants(r.Context(), filter)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	items := make([]TenantResponse, 0, len(result.Tenants))
	for _, t := range result.Tenants {
		items = append(items, toResponse(t))
	}
	httpx.JSON(w, http.StatusOK, httpx.Page[TenantResponse]{Items: items, NextCursor: result.NextCursor})
}

func (h *Handlers) ChangeStatus(w http.ResponseWriter, r *http.Request) {
	var req ChangeStatusRequest
	if !httpx.Decode(w, r, &req) {
		return
	}
	tenant, err := h.commands.ChangeStatus(r.Context(), app.ChangeStatusInput{
		TenantID: chi.URLParam(r, "id"), Status: req.Status, ExpectedVersion: ifMatch(r),
	})
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	setETag(w, tenant.Version())
	httpx.JSON(w, http.StatusOK, toResponse(tenant))
}

func (h *Handlers) UpdateQuota(w http.ResponseWriter, r *http.Request) {
	var req UpdateQuotaRequest
	if !httpx.Decode(w, r, &req) {
		return
	}
	tenant, err := h.commands.UpdateQuota(r.Context(), app.UpdateQuotaInput{
		TenantID: chi.URLParam(r, "id"), MaxServices: req.MaxServices, ExpectedVersion: ifMatch(r),
	})
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	setETag(w, tenant.Version())
	httpx.JSON(w, http.StatusOK, toResponse(tenant))
}

func parseListFilter(r *http.Request) (app.ListFilter, error) {
	q := r.URL.Query()
	f := app.ListFilter{Order: app.OrderDesc, Limit: 50, Cursor: q.Get("cursor")}
	if v := q.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			return f, apperr.Validation("INVALID_LIMIT", "limit must be a positive integer")
		}
		f.Limit = n
	}
	if v := q.Get("status"); v != "" {
		st, err := domain.ParseStatus(v)
		if err != nil {
			return f, err
		}
		f.Status = &st
	}
	if v := q.Get("plan"); v != "" {
		p, err := domain.ParsePlan(v)
		if err != nil {
			return f, err
		}
		f.Plan = &p
	}
	switch q.Get("order") {
	case "", "desc":
		f.Order = app.OrderDesc
	case "asc":
		f.Order = app.OrderAsc
	default:
		return f, apperr.Validation("INVALID_ORDER", "order must be asc or desc")
	}
	return f, nil
}

func ifMatch(r *http.Request) int64 {
	raw := strings.Trim(r.Header.Get("If-Match"), `"`)
	if raw == "" {
		return 0
	}
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0
	}
	return v
}

func setETag(w http.ResponseWriter, version int64) {
	w.Header().Set("ETag", strconv.FormatInt(version, 10))
}
