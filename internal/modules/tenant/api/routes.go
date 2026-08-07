package api

import "github.com/go-chi/chi/v5"

func (h *Handlers) Mount(r chi.Router) {
	r.Route("/tenants", func(r chi.Router) {
		r.Post("/", h.Create)
		r.Get("/", h.List)
		r.Get("/{id}", h.Get)
		r.Patch("/{id}/status", h.ChangeStatus)
		r.Patch("/{id}/quota", h.UpdateQuota)
	})
}
