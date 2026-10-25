// Command users is the gRPC-only users microservice. It owns the users business
// rules and the database; the HTTP gateway reaches it over gRPC.
package main

import (
	_ "time/tzdata"

	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
	"go.uber.org/zap"

	"github.com/qvarkk/go-fiber-template/backend/internal/config"
	"github.com/qvarkk/go-fiber-template/backend/internal/db"
	"github.com/qvarkk/go-fiber-template/backend/internal/grpcserver"
	"github.com/qvarkk/go-fiber-template/backend/internal/logger"
	"github.com/qvarkk/go-fiber-template/backend/internal/user"
)

var version = "dev"

func main() {
	fx.New(
		config.Module,
		logger.Module,
		db.Module,
		grpcserver.Module,
		user.Module,

		fx.WithLogger(func(cfg *config.Config, log *zap.Logger) fxevent.Logger {
			if cfg.App.Env.IsDevelopment() {
				return &fxevent.ZapLogger{Logger: log.Named("fx")}
			}
			return fxevent.NopLogger
		}),

		fx.Invoke(reportStartup),
	).Run()
}

// reportStartup runs while the graph is being built, before the OnStart hooks.
func reportStartup(cfg *config.Config, log *zap.Logger) {
	log.Info("application constructed, starting server",
		zap.String("version", version),
		zap.String("name", cfg.App.Name),
		zap.String("env", string(cfg.App.Env)),
	)
}
