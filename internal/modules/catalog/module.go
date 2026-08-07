package catalog

import (
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/forge-platform/forge/internal/modules/catalog/adapters/postgres"
	"github.com/forge-platform/forge/internal/modules/catalog/api"
	"github.com/forge-platform/forge/internal/modules/catalog/app"
	"github.com/forge-platform/forge/internal/platform/idempotency"
)

// Module wires the catalog bounded context. It composes the persistence
// adapters, application services, and HTTP handlers, exposing only a Mount
// method so the composition root stays ignorant of internal structure.
type Module struct {
	handlers *api.Handlers
}

func New(pool *pgxpool.Pool, idem *idempotency.Store) *Module {
	store := postgres.NewStore(pool)
	commands := app.NewCommands(store, app.SystemClock{})
	queries := app.NewQueries(store)
	return &Module{handlers: api.NewHandlers(commands, queries, idem)}
}

func (m *Module) Mount(r chi.Router) { m.handlers.Mount(r) }
