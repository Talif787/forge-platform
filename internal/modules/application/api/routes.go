package api

import "github.com/go-chi/chi/v5"

func (h *Handlers) Mount(r chi.Router) {
	r.Route("/applications", func(r chi.Router) {
		r.Get("/", h.List)
		r.Get("/{namespace}/{name}", h.Get)
	})
}
