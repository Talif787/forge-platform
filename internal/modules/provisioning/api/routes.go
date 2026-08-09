package api

import "github.com/go-chi/chi/v5"

func (h *Handlers) Mount(r chi.Router) {
	r.Route("/provisioning", func(r chi.Router) {
		r.Get("/workflows", h.List)
		r.Get("/workflows/{workflowId}", h.Get)
	})
}
