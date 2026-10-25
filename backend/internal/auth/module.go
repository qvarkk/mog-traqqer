package auth

import (
	"go.uber.org/fx"
	"google.golang.org/grpc"

	authv1 "github.com/qvarkk/mog-traqqer/backend/gen/auth/v1"
)

// Module wires the auth service into the FX graph. It belongs to cmd/users: the
// HTTP gateway reaches this service over gRPC and shares none of this code.
var Module = fx.Module("auth",
	fx.Provide(NewService),
	fx.Provide(NewGRPCHandler),
	fx.Invoke(registerGRPCService),
)

func registerGRPCService(server *grpc.Server, handler *GRPCHandler) {
	authv1.RegisterAuthServiceServer(server, handler)
}
