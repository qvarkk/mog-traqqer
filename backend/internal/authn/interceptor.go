package authn

import (
	"context"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const authorizationHeader = "authorization"

// PublicMethods lists the RPCs that need no access token: their credential is in
// the request body (register, login, refresh, logout).
func PublicMethods() map[string]bool {
	return map[string]bool{
		"/auth.v1.AuthService/Register": true,
		"/auth.v1.AuthService/Login":    true,
		"/auth.v1.AuthService/Refresh":  true,
		"/auth.v1.AuthService/Logout":   true,
	}
}

// UnaryAuth rejects unauthenticated calls and injects the authenticated user id
// into the context of the rest.
func UnaryAuth(secret string, public map[string]bool) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		if public[info.FullMethod] {
			return handler(ctx, req)
		}

		md, _ := metadata.FromIncomingContext(ctx)
		values := md.Get(authorizationHeader)
		if len(values) == 0 {
			return nil, status.Error(codes.Unauthenticated, "missing credentials")
		}

		userID, err := Verify(secret, strings.TrimPrefix(values[0], "Bearer "))
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, "invalid or expired token")
		}

		return handler(WithUserID(ctx, userID), req)
	}
}
