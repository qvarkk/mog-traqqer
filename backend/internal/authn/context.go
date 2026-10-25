package authn

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
)

type userIDKey struct{}

// WithUserID stores the authenticated user id in the context.
func WithUserID(ctx context.Context, id pgtype.UUID) context.Context {
	return context.WithValue(ctx, userIDKey{}, id)
}

// UserIDFromContext returns the authenticated user id, if the interceptor set it.
func UserIDFromContext(ctx context.Context) (pgtype.UUID, bool) {
	id, ok := ctx.Value(userIDKey{}).(pgtype.UUID)
	return id, ok
}
