package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/forge-platform/forge/internal/modules/catalog/app"
	"github.com/forge-platform/forge/internal/modules/catalog/domain"
	"github.com/forge-platform/forge/internal/platform/apperr"
	"github.com/forge-platform/forge/internal/platform/httpx"
	"github.com/forge-platform/forge/internal/platform/idempotency"
)

type Handlers struct {
	commands *app.Commands
	queries  *app.Queries
	idem     *idempotency.Store
}

func NewHandlers(commands *app.Commands, queries *app.Queries, idem *idempotency.Store) *Handlers {
	return &Handlers{commands: commands, queries: queries, idem: idem}
}

func (h *Handlers) Register(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		httpx.Error(w, r, apperr.Validation("INVALID_BODY", "request body could not be read"))
		return
	}
	var req RegisterServiceRequest
	if err := json.Unmarshal(body, &req); err != nil {
		httpx.Error(w, r, apperr.Validation("INVALID_BODY", "request body is not valid JSON"))
		return
	}

	idemKey := r.Header.Get("Idempotency-Key")
	if idemKey != "" && h.idem != nil {
		if rec, found, err := h.idem.Lookup(r.Context(), idemKey); err != nil {
			httpx.Error(w, r, err)
			return
		} else if found {
			w.Header().Set("Idempotency-Replayed", "true")
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(rec.Status)
			_, _ = w.Write(rec.Body)
			return
		}
	}

	svc, err := h.commands.RegisterService(r.Context(), app.RegisterServiceInput{
		TenantID:    req.TenantID,
		Name:        req.Name,
		Description: req.Description,
		Tier:        req.Tier,
		Repository:  req.Repository,
		OwningTeam:  req.OwningTeam,
		OnCallRef:   req.OnCallRef,
	})
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	resp := toResponse(svc)
	if idemKey != "" && h.idem != nil {
		stored, _ := json.Marshal(resp)
		sum := sha256.Sum256(body)
		_ = h.idem.Save(r.Context(), idemKey, hex.EncodeToString(sum[:]), http.StatusCreated, stored)
	}
	w.Header().Set("Location", "/api/v1/services/"+svc.ID().String())
	setETag(w, svc.Version())
	httpx.JSON(w, http.StatusCreated, resp)
}

func (h *Handlers) Get(w http.ResponseWriter, r *http.Request) {
	svc, err := h.queries.GetService(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	setETag(w, svc.Version())
	httpx.JSON(w, http.StatusOK, toResponse(svc))
}

func (h *Handlers) List(w http.ResponseWriter, r *http.Request) {
	filter, err := parseListFilter(r)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	result, err := h.queries.ListServices(r.Context(), filter)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	items := make([]ServiceResponse, 0, len(result.Services))
	for _, s := range result.Services {
		items = append(items, toResponse(s))
	}
	httpx.JSON(w, http.StatusOK, httpx.Page[ServiceResponse]{Items: items, NextCursor: result.NextCursor})
}

func (h *Handlers) ChangeOwnership(w http.ResponseWriter, r *http.Request) {
	var req ChangeOwnershipRequest
	if !httpx.Decode(w, r, &req) {
		return
	}
	svc, err := h.commands.ChangeOwnership(r.Context(), app.ChangeOwnershipInput{
		ServiceID:       chi.URLParam(r, "id"),
		OwningTeam:      req.OwningTeam,
		OnCallRef:       req.OnCallRef,
		ExpectedVersion: ifMatch(r),
	})
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	setETag(w, svc.Version())
	httpx.JSON(w, http.StatusOK, toResponse(svc))
}

func (h *Handlers) ChangeLifecycle(w http.ResponseWriter, r *http.Request) {
	var req ChangeLifecycleRequest
	if !httpx.Decode(w, r, &req) {
		return
	}
	svc, err := h.commands.ChangeLifecycle(r.Context(), app.ChangeLifecycleInput{
		ServiceID:       chi.URLParam(r, "id"),
		Lifecycle:       req.Lifecycle,
		ExpectedVersion: ifMatch(r),
	})
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	setETag(w, svc.Version())
	httpx.JSON(w, http.StatusOK, toResponse(svc))
}

func (h *Handlers) Retire(w http.ResponseWriter, r *http.Request) {
	svc, err := h.commands.RetireService(r.Context(), app.RetireServiceInput{
		ServiceID:       chi.URLParam(r, "id"),
		ExpectedVersion: ifMatch(r),
	})
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	setETag(w, svc.Version())
	httpx.JSON(w, http.StatusOK, toResponse(svc))
}

func parseListFilter(r *http.Request) (app.ListFilter, error) {
	q := r.URL.Query()
	f := app.ListFilter{Sort: app.SortCreatedAt, Order: app.OrderDesc, Limit: 50, Cursor: q.Get("cursor")}

	if v := q.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			return f, apperr.Validation("INVALID_LIMIT", "limit must be a positive integer")
		}
		f.Limit = n
	}
	if v := q.Get("tenantId"); v != "" {
		tid, err := domain.TenantIDFromString(v)
		if err != nil {
			return f, err
		}
		f.TenantID = &tid
	}
	if v := q.Get("lifecycle"); v != "" {
		lc, err := domain.ParseLifecycle(v)
		if err != nil {
			return f, err
		}
		f.Lifecycle = &lc
	}
	if v := q.Get("tier"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return f, apperr.Validation("INVALID_TIER", "tier must be an integer from 1 to 4")
		}
		tier, err := domain.NewTier(n)
		if err != nil {
			return f, err
		}
		f.Tier = &tier
	}
	switch q.Get("sort") {
	case "", "created_at":
		f.Sort = app.SortCreatedAt
	case "name":
		f.Sort = app.SortName
	default:
		return f, apperr.Validation("INVALID_SORT", "sort must be created_at or name")
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
