package grpcserver

import (
	"context"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// UnaryLogger must sit outside UnaryRecovery so a recovered panic still reaches
// it as an error.
func UnaryLogger(log *zap.Logger) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		request any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		start := time.Now()

		response, err := handler(ctx, request)

		code := status.Code(err)
		fields := []zap.Field{
			zap.String("method", info.FullMethod),
			zap.String("code", code.String()),
			zap.Duration("duration", time.Since(start)),
		}

		switch {
		case isServerFault(code):
			log.Error("grpc request", append(fields, zap.Error(err))...)
		case code != codes.OK:
			log.Warn("grpc request", fields...)
		default:
			log.Info("grpc request", fields...)
		}

		return response, err
	}
}

// UnaryRecovery turns a panic into an Internal status; the panic and stack go
// to the log, never to the client.
func UnaryRecovery(log *zap.Logger) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		request any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (response any, err error) {
		defer func() {
			if recovered := recover(); recovered != nil {
				log.Error("panic in grpc handler",
					zap.String("method", info.FullMethod),
					zap.Any("panic", recovered),
					zap.Stack("stack"),
				)
				err = status.Error(codes.Internal, "internal error")
			}
		}()

		return handler(ctx, request)
	}
}
