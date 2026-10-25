package gateway

import (
	"github.com/gofiber/fiber/v3"
	"go.uber.org/fx"

	"github.com/qvarkk/mog-traqqer/backend/internal/server"
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
	auth := app.Group("/api/v1/auth")

	auth.Post("/register", handler.Register)
	auth.Post("/login", handler.Login)
	auth.Post("/refresh", handler.Refresh)
	auth.Post("/logout", handler.Logout)
	auth.Get("/me", handler.Me)
}
