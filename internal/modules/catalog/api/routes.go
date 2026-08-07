package api

import "github.com/go-chi/chi/v5"

func (h *Handlers) Mount(r chi.Router) {
	r.Route("/services", func(r chi.Router) {
		r.Post("/", h.Register)
		r.Get("/", h.List)
		r.Get("/{id}", h.Get)
		r.Patch("/{id}/ownership", h.ChangeOwnership)
		r.Patch("/{id}/lifecycle", h.ChangeLifecycle)
		r.Delete("/{id}", h.Retire)
	})
}
