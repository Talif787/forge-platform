package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/forge-platform/forge/internal/modules/provisioning/app"
	"github.com/forge-platform/forge/internal/platform/httpx"
)

type Handlers struct {
	reader app.Reader
}

func NewHandlers(reader app.Reader) *Handlers { return &Handlers{reader: reader} }

func (h *Handlers) List(w http.ResponseWriter, r *http.Request) {
	views, err := h.reader.List(r.Context())
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	items := make([]WorkflowResponse, 0, len(views))
	for _, v := range views {
		items = append(items, toResponse(v))
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handlers) Get(w http.ResponseWriter, r *http.Request) {
	view, err := h.reader.Get(r.Context(), chi.URLParam(r, "workflowId"))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, toResponse(view))
}
