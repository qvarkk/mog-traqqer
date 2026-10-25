package server

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/requestid"
)

const readinessTimeout = 2 * time.Second

// Pinger is the dependency check behind /readyz, declared by the consumer so
// this package never imports the gateway's client.
type Pinger interface {
	Ping(ctx context.Context) error
}

func registerRoutes(app *fiber.App, pinger Pinger) {
	app.Get("/healthz", health)
	app.Get("/readyz", ready(pinger))

	v1 := app.Group("/api/v1")
	v1.Get("/ping", ping)
}

// health is liveness only. It must not check dependencies: an unreachable
// dependency should make the instance unready, not make the orchestrator kill
// and restart it.
func health(c fiber.Ctx) error {
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "ok"})
}

// ready checks the users service and returns 503 while it is unreachable. The
// failure detail is logged by the error handler rather than returned, since
// /readyz is often reachable from outside the cluster.
func ready(pinger Pinger) fiber.Handler {
	return func(c fiber.Ctx) error {
		ctx, cancel := context.WithTimeout(c.Context(), readinessTimeout)
		defer cancel()

		if err := pinger.Ping(ctx); err != nil {
			return NewAPIError(
				fiber.StatusServiceUnavailable,
				"users_service_unavailable",
				"the users service is not reachable",
			).Wrap(err)
		}

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"status": "ready",
			"checks": fiber.Map{"users": "ok"},
		})
	}
}

func ping(c fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"message":    "pong",
		"request_id": requestid.FromContext(c),
	})
}
