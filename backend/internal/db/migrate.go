package db

import (
	"context"
	"embed"
	"fmt"
	"io/fs"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"
	"go.uber.org/zap"

	"github.com/qvarkk/go-fiber-template/backend/internal/config"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// RunMigrations runs during FX construction, not an OnStart hook, so the schema
// is guaranteed ready before any request is served.
func RunMigrations(cfg *config.Config, pool *pgxpool.Pool, log *zap.Logger) error {
	if !cfg.DB.AutoMigrate {
		log.Info("database migrations disabled", zap.String("reason", "DB_AUTO_MIGRATE=false"))
		return nil
	}

	// goose works over database/sql; closing the wrapper does not close the pool.
	sqlDB := stdlib.OpenDBFromPool(pool)

	// The advisory lock lets several replicas boot at once without racing.
	locker, err := lock.NewPostgresSessionLocker()
	if err != nil {
		_ = sqlDB.Close()
		return fmt.Errorf("create migration session locker: %w", err)
	}

	// go:embed keeps the migrations/ prefix, but goose expects the files at the
	// root of the filesystem it is given.
	migrationFS, err := fs.Sub(migrationFiles, "migrations")
	if err != nil {
		_ = sqlDB.Close()
		return fmt.Errorf("open embedded migrations: %w", err)
	}

	provider, err := goose.NewProvider(goose.DialectPostgres, sqlDB, migrationFS,
		goose.WithSessionLocker(locker),
	)
	if err != nil {
		_ = sqlDB.Close()
		return fmt.Errorf("create migration provider: %w", err)
	}
	defer func() { _ = provider.Close() }()

	results, err := provider.Up(context.Background())
	if err != nil {
		return fmt.Errorf("apply database migrations: %w", err)
	}

	if len(results) == 0 {
		log.Info("database schema is up to date")
		return nil
	}

	for _, result := range results {
		log.Info("migration applied",
			zap.String("migration", result.Source.Path),
			zap.Int64("version", result.Source.Version),
			zap.Duration("duration", result.Duration),
		)
	}

	return nil
}
