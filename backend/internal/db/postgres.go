// Package db owns the PostgreSQL pool and schema. The schema lives in
// migrations/*.sql: goose applies it and sqlc reads it, so it has one source.
package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/fx"
	"go.uber.org/zap"

	"github.com/qvarkk/mog-traqqer/backend/internal/config"
)

func NewPool(lc fx.Lifecycle, cfg *config.Config, log *zap.Logger) (*pgxpool.Pool, error) {
	poolConfig, err := pgxpool.ParseConfig(cfg.DB.DSN())
	if err != nil {
		return nil, fmt.Errorf("parse database connection string: %w", err)
	}

	poolConfig.MaxConns = cfg.DB.MaxConns
	poolConfig.MinConns = cfg.DB.MinConns
	poolConfig.MaxConnLifetime = cfg.DB.MaxConnLifetime
	poolConfig.MaxConnIdleTime = cfg.DB.MaxConnIdleTime
	poolConfig.ConnConfig.ConnectTimeout = cfg.DB.ConnectTimeout

	ctx, cancel := context.WithTimeout(context.Background(), cfg.DB.ConnectTimeout)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("create database connection pool: %w", err)
	}

	// Ping once so a wrong password or unreachable host fails startup, which in
	// an orchestrator means the container restarts until the database is up.
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database at %s: %w", cfg.DB.SafeDSN(), err)
	}

	log.Info("connected to database",
		zap.String("database", cfg.DB.SafeDSN()),
		zap.Int32("max_conns", cfg.DB.MaxConns),
	)

	lc.Append(fx.Hook{
		OnStop: func(context.Context) error {
			// Close waits for in-flight queries. Listed before grpcserver, this
			// hook runs after the gRPC listener has stopped.
			pool.Close()
			return nil
		},
	})

	return pool, nil
}
