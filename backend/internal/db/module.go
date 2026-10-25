package db

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/fx"

	"github.com/qvarkk/mog-traqqer/backend/internal/db/dbgen"
)

// Module wires PostgreSQL into the FX graph. It belongs to cmd/users: the pool
// must be registered before the gRPC server so it closes after the listener
// stops.
var Module = fx.Module("db",
	fx.Provide(NewPool),
	fx.Provide(newQuerier),
	fx.Invoke(RunMigrations),
)

func newQuerier(pool *pgxpool.Pool) dbgen.Querier {
	return dbgen.New(pool)
}
