// Package grpcserver hosts the gRPC transport: its own listener and port,
// managed by its own lifecycle hooks. Services register against the *grpc.Server
// this package provides.
package grpcserver

import (
	"context"
	"fmt"
	"net"
	"sync/atomic"
	"time"

	"go.uber.org/fx"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	"github.com/qvarkk/mog-traqqer/backend/internal/config"
)

func New(lc fx.Lifecycle, cfg *config.Config, log *zap.Logger) *grpc.Server {
	server := grpc.NewServer(
		grpc.MaxRecvMsgSize(cfg.GRPC.MaxRecvMsgSize),
		grpc.MaxSendMsgSize(cfg.GRPC.MaxSendMsgSize),
		grpc.ConnectionTimeout(cfg.GRPC.ConnectionTimeout),
		// The logger sits outside the recovery interceptor so a recovered panic
		// is still logged instead of unwinding past it.
		grpc.ChainUnaryInterceptor(
			UnaryLogger(log),
			UnaryRecovery(log),
		),
	)

	// The standard health service is what grpc_health_probe and Kubernetes gRPC
	// readiness probes call.
	healthServer := health.NewServer()
	healthpb.RegisterHealthServer(server, healthServer)

	if cfg.GRPC.ReflectionEnabled {
		reflection.Register(server)
	}

	var shuttingDown atomic.Bool

	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			if !cfg.GRPC.Enabled {
				log.Info("grpc server disabled", zap.String("reason", "GRPC_ENABLED=false"))
				return nil
			}

			listener, err := net.Listen("tcp", cfg.GRPC.Addr())
			if err != nil {
				return fmt.Errorf("listen for grpc on %s: %w", cfg.GRPC.Addr(), err)
			}

			log.Info("grpc server starting",
				zap.String("addr", cfg.GRPC.Addr()),
				zap.Bool("reflection", cfg.GRPC.ReflectionEnabled),
			)

			go func() {
				if err := server.Serve(listener); err != nil && !shuttingDown.Load() {
					log.Error("grpc server stopped unexpectedly", zap.Error(err))
				}
			}()

			return nil
		},

		OnStop: func(context.Context) error {
			if !cfg.GRPC.Enabled {
				return nil
			}

			shuttingDown.Store(true)

			log.Info("grpc server shutting down",
				zap.Duration("timeout", cfg.GRPC.ShutdownTimeout),
			)

			// Report NOT_SERVING before draining so a load balancer stops
			// routing here while in-flight calls finish.
			healthServer.SetServingStatus("", healthpb.HealthCheckResponse_NOT_SERVING)

			// GracefulStop can block forever on a stuck stream, so bound it.
			drained := make(chan struct{})
			go func() {
				server.GracefulStop()
				close(drained)
			}()

			select {
			case <-drained:
			case <-time.After(cfg.GRPC.ShutdownTimeout):
				log.Warn("grpc graceful shutdown timed out; forcing stop")
				server.Stop()
			}

			healthServer.Shutdown()

			return nil
		},
	})

	return server
}
