package gateway

import (
	"github.com/gofiber/fiber/v3"
	"go.uber.org/fx"

	"github.com/qvarkk/go-fiber-template/backend/internal/server"
)

// Module wires the HTTP gateway into the FX graph. It belongs to cmd/gateway.
var Module = fx.Module("gateway",
	fx.Provide(NewClient),
	fx.Provide(NewHandler),
	fx.Provide(newPinger),
	fx.Invoke(registerRoutes),
)

func newPinger(client *Client) server.Pinger {
	return client
}

func registerRoutes(app *fiber.App, handler *Handler) {
	users := app.Group("/api/v1/users")

	users.Post("/", handler.Create)
	users.Get("/", handler.List)
	users.Get("/:id", handler.Get)
	users.Patch("/:id", handler.Update)
	users.Delete("/:id", handler.Delete)
}
