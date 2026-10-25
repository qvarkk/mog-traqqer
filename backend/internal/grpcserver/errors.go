package grpcserver

import (
	"go.uber.org/zap"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Err builds a status error carrying a stable machine reason. The reason is
// what a gateway maps onto an HTTP error code, and what makes errors
// translatable, so it must never be a human sentence.
//
// The cause is logged only when the failure is ours: a status error cannot wrap
// a cause, so without this the underlying database error would be lost.
func Err(log *zap.Logger, code codes.Code, reason, message string, cause error) error {
	st := status.New(code, message)
	if reason != "" {
		st, _ = st.WithDetails(&errdetails.ErrorInfo{Reason: reason})
	}

	if cause != nil && isServerFault(code) {
		log.Error("grpc handler failed",
			zap.String("code", code.String()),
			zap.String("reason", reason),
			zap.Error(cause),
		)
	}

	return st.Err()
}

// FieldViolation is one field that failed validation.
type FieldViolation struct {
	// Field is the JSON field name the client sees, such as "email".
	Field string
	// Key is the translation key the gateway uses to localise the message.
	Key string
	// Description is the English fallback served when no catalog has Key.
	Description string
}

// InvalidArgument builds an INVALID_ARGUMENT status error listing the offending
// fields. reason is the envelope code ("validation_failed"); each violation's
// Key and Description ride along so the gateway can translate them.
func InvalidArgument(reason string, violations []FieldViolation) error {
	st := status.New(codes.InvalidArgument, "invalid request")

	info := &errdetails.ErrorInfo{Reason: reason, Metadata: map[string]string{}}
	badRequest := &errdetails.BadRequest{}

	for _, v := range violations {
		badRequest.FieldViolations = append(badRequest.FieldViolations, &errdetails.BadRequest_FieldViolation{
			Field:       v.Field,
			Description: v.Description,
		})
		if v.Key != "" {
			info.Metadata[v.Field] = v.Key
		}
	}

	st, _ = st.WithDetails(info, badRequest)
	return st.Err()
}

// isServerFault reports whether a status code means the failure is ours.
func isServerFault(code codes.Code) bool {
	switch code {
	case codes.Internal,
		codes.Unknown,
		codes.Unavailable,
		codes.DataLoss,
		codes.DeadlineExceeded:
		return true
	default:
		return false
	}
}
