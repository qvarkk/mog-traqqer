package grpcserver

import "go.uber.org/fx"

// Module must be listed after db.Module: FX stops hooks in reverse construction
// order, so registering the pool first closes it after this listener stops.
var Module = fx.Module("grpc-server", fx.Provide(New))
