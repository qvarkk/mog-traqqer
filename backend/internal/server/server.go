// Package server hosts the HTTP transport: the Fiber app, its middleware chain,
// the error-to-response mapping and the listener lifecycle. It belongs to the
// gateway.
package server

import (
	"context"
	"sync/atomic"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/gofiber/fiber/v3/middleware/requestid"
	"go.uber.org/fx"
	"go.uber.org/zap"

	"github.com/qvarkk/go-fiber-template/backend/internal/config"
	"github.com/qvarkk/go-fiber-template/backend/internal/i18n"
)

var Module = fx.Module("http-server", fx.Provide(New))

// New builds the Fiber app and registers its lifecycle hooks. It is returned so
// any module can inject it to add middleware or routes.
//
// It depends on the readiness Pinger so that the prober is constructed first
// and therefore closed after this server drains on shutdown.
func New(lc fx.Lifecycle, cfg *config.Config, log *zap.Logger, bundle *i18n.Bundle, pinger Pinger) *fiber.App {
	app := fiber.New(fiber.Config{
		AppName:      cfg.App.Name,
		ReadTimeout:  cfg.HTTP.ReadTimeout,
		WriteTimeout: cfg.HTTP.WriteTimeout,
		IdleTimeout:  cfg.HTTP.IdleTimeout,
		BodyLimit:    cfg.HTTP.BodyLimit,
		ErrorHandler: errorHandler(log),
	})

	app.Use(cors.New(cors.Config{AllowOrigins: cfg.HTTP.CORSAllowedOrigins}))
	app.Use(requestid.New())
	app.Use(i18n.Middleware(bundle))
	// The access log sits outside recover so a panic still unwinds through it
	// and gets logged.
	app.Use(RequestLogger(log))
	app.Use(recover.New(recover.Config{EnableStackTrace: true}))

	registerRoutes(app, pinger)

	var shuttingDown atomic.Bool

	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			addr := cfg.HTTP.Addr()

			log.Info("http server starting",
				zap.String("addr", addr),
				zap.String("env", string(cfg.App.Env)),
			)

			go func() {
				err := app.Listen(addr, fiber.ListenConfig{DisableStartupMessage: true})
				if err != nil && !shuttingDown.Load() {
					log.Error("http server stopped unexpectedly", zap.Error(err))
				}
			}()

			return nil
		},

		OnStop: func(context.Context) error {
			shuttingDown.Store(true)

			log.Info("http server shutting down",
				zap.Duration("timeout", cfg.HTTP.ShutdownTimeout),
			)

			// Give in-flight requests a chance to finish; expiring the grace
			// period is worth logging but not a reason to block shutdown.
			if err := app.ShutdownWithTimeout(cfg.HTTP.ShutdownTimeout); err != nil {
				log.Warn("http server shutdown did not complete cleanly", zap.Error(err))
			}

			return nil
		},
	})

	return app
}
