package gateway

import (
	"go.uber.org/fx"

	"github.com/qvarkk/mog-traqqer/backend/internal/server"
)

// Module wires the HTTP gateway into the FX graph. It belongs to cmd/gateway.
// Phase 2/3 add per-domain handlers and their route registration here.
var Module = fx.Module("gateway",
	fx.Provide(NewClient),
	fx.Provide(newPinger),
)

func newPinger(client *Client) server.Pinger {
	return client
}
