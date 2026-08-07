package tenant

import (
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/forge-platform/forge/internal/modules/tenant/adapters/postgres"
	"github.com/forge-platform/forge/internal/modules/tenant/api"
	"github.com/forge-platform/forge/internal/modules/tenant/app"
)

type Module struct {
	handlers *api.Handlers
}

func New(pool *pgxpool.Pool) *Module {
	store := postgres.NewStore(pool)
	commands := app.NewCommands(store, app.SystemClock{})
	queries := app.NewQueries(store)
	return &Module{handlers: api.NewHandlers(commands, queries)}
}

func (m *Module) Mount(r chi.Router) { m.handlers.Mount(r) }
