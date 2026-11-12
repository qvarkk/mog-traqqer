package catalog

import (
	"go.uber.org/fx"
	"google.golang.org/grpc"

	catalogv1 "github.com/qvarkk/mog-traqqer/backend/gen/catalog/v1"
)

// Module wires the catalog service into the FX graph. It belongs to cmd/users.
var Module = fx.Module("catalog",
	fx.Provide(NewService),
	fx.Provide(NewGRPCHandler),
	fx.Invoke(registerGRPCService),
)

func registerGRPCService(server *grpc.Server, handler *GRPCHandler) {
	catalogv1.RegisterCatalogServiceServer(server, handler)
}
