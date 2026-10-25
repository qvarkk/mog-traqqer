package user

import (
	"go.uber.org/fx"
	"google.golang.org/grpc"

	userv1 "github.com/qvarkk/go-fiber-template/backend/gen/user/v1"
)

// Module wires the users service into the FX graph. It belongs to cmd/users:
// the HTTP gateway reaches this service over gRPC and shares none of this code.
var Module = fx.Module("user",
	fx.Provide(NewService),
	fx.Provide(NewGRPCHandler),
	fx.Invoke(registerGRPCService),
)

func registerGRPCService(server *grpc.Server, handler *GRPCHandler) {
	userv1.RegisterUserServiceServer(server, handler)
}
