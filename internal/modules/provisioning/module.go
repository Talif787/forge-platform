package provisioning

import (
	"github.com/go-chi/chi/v5"

	"github.com/forge-platform/forge/internal/modules/provisioning/api"
	"github.com/forge-platform/forge/internal/modules/provisioning/app"
)

type Module struct {
	handlers *api.Handlers
}

// New builds the module backed by a live reader (a Temporal client).
func New(reader app.Reader) *Module {
	return &Module{handlers: api.NewHandlers(reader)}
}

// NewDisabled builds the module in its degraded state: endpoints respond 503.
func NewDisabled() *Module {
	return &Module{handlers: api.NewHandlers(app.DisabledReader{})}
}

func (m *Module) Mount(r chi.Router) { m.handlers.Mount(r) }
