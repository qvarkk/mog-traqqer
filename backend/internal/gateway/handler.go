package gateway

import (
	"time"

	"github.com/gofiber/fiber/v3"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	authv1 "github.com/qvarkk/mog-traqqer/backend/gen/auth/v1"
	"github.com/qvarkk/mog-traqqer/backend/internal/i18n"
	"github.com/qvarkk/mog-traqqer/backend/internal/server"
)

// Handler translates HTTP requests into auth-service calls. It owns no business
// rules: it binds JSON, dials the client and maps the gRPC result onto the HTTP
// envelope.
type Handler struct {
	auth authv1.AuthServiceClient
}

func NewHandler(client *Client) *Handler {
	return &Handler{auth: client.Auth}
}

type credentialsRequest struct {
	Email      string `json:"email"`
	Password   string `json:"password"`
	DeviceInfo string `json:"device_info"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

func (h *Handler) Register(c fiber.Ctx) error {
	var req credentialsRequest
	if err := c.Bind().Body(&req); err != nil {
		return server.BadRequest("invalid_body", "the request body could not be parsed as JSON").Wrap(err)
	}

	resp, err := h.auth.Register(c.Context(), &authv1.RegisterRequest{
		Email:      req.Email,
		Password:   req.Password,
		DeviceInfo: req.DeviceInfo,
	})
	if err != nil {
		return fromGRPC(c, err)
	}

	return c.Status(fiber.StatusCreated).JSON(tokensJSON(resp.GetAccessToken(), resp.GetRefreshToken(), resp.GetUser()))
}

func (h *Handler) Login(c fiber.Ctx) error {
	var req credentialsRequest
	if err := c.Bind().Body(&req); err != nil {
		return server.BadRequest("invalid_body", "the request body could not be parsed as JSON").Wrap(err)
	}

	resp, err := h.auth.Login(c.Context(), &authv1.LoginRequest{
		Email:      req.Email,
		Password:   req.Password,
		DeviceInfo: req.DeviceInfo,
	})
	if err != nil {
		return fromGRPC(c, err)
	}

	return c.JSON(tokensJSON(resp.GetAccessToken(), resp.GetRefreshToken(), resp.GetUser()))
}

func (h *Handler) Refresh(c fiber.Ctx) error {
	var req refreshRequest
	if err := c.Bind().Body(&req); err != nil {
		return server.BadRequest("invalid_body", "the request body could not be parsed as JSON").Wrap(err)
	}

	resp, err := h.auth.Refresh(c.Context(), &authv1.RefreshRequest{RefreshToken: req.RefreshToken})
	if err != nil {
		return fromGRPC(c, err)
	}

	return c.JSON(tokensJSON(resp.GetAccessToken(), resp.GetRefreshToken(), resp.GetUser()))
}

func (h *Handler) Logout(c fiber.Ctx) error {
	var req refreshRequest
	if err := c.Bind().Body(&req); err != nil {
		return server.BadRequest("invalid_body", "the request body could not be parsed as JSON").Wrap(err)
	}

	if _, err := h.auth.Logout(c.Context(), &authv1.LogoutRequest{RefreshToken: req.RefreshToken}); err != nil {
		return fromGRPC(c, err)
	}

	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Handler) Me(c fiber.Ctx) error {
	ctx := c.Context()
	if token := c.Get("Authorization"); token != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, "authorization", token)
	}

	resp, err := h.auth.GetMe(ctx, &authv1.GetMeRequest{})
	if err != nil {
		return fromGRPC(c, err)
	}

	return c.JSON(userJSON(resp.GetUser()))
}

func tokensJSON(accessToken, refreshToken string, user *authv1.User) map[string]any {
	return map[string]any{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"user":          userJSON(user),
	}
}

func userJSON(u *authv1.User) map[string]any {
	return map[string]any{
		"id":         u.GetId(),
		"email":      u.GetEmail(),
		"created_at": formatTime(u.GetCreatedAt()),
		"updated_at": formatTime(u.GetUpdatedAt()),
	}
}

func formatTime(t *timestamppb.Timestamp) string {
	if t == nil {
		return ""
	}
	return t.AsTime().UTC().Format(time.RFC3339Nano)
}

// fromGRPC maps an auth-service status onto an APIError, so the error envelope
// and its i18n behave exactly as when the logic lived in this process.
func fromGRPC(c fiber.Ctx, err error) error {
	st, ok := status.FromError(err)
	if !ok {
		return server.Internal("upstream_error", "unexpected error from the upstream service").Wrap(err)
	}

	reason, meta, violations := extractDetails(st)

	switch st.Code() {
	case codes.InvalidArgument:
		if reason == "validation_failed" {
			return validationError(c, meta, violations)
		}
		return server.BadRequest(codeOr(reason, "invalid_argument"), st.Message())
	case codes.NotFound:
		return server.NotFound(codeOr(reason, "not_found"), st.Message())
	case codes.AlreadyExists:
		return server.Conflict(codeOr(reason, "conflict"), st.Message())
	case codes.PermissionDenied:
		return server.Forbidden(codeOr(reason, "forbidden"), st.Message())
	case codes.Unauthenticated:
		return server.Unauthorized(codeOr(reason, "unauthorized"), st.Message())
	case codes.Unavailable, codes.DeadlineExceeded:
		return server.NewAPIError(fiber.StatusServiceUnavailable, codeOr(reason, "service_unavailable"), st.Message())
	default:
		return server.Internal(codeOr(reason, "internal_error"), st.Message()).Wrap(err)
	}
}

func validationError(c fiber.Ctx, meta map[string]string, violations []*errdetails.BadRequest_FieldViolation) error {
	localizer := i18n.FromContext(c)
	fields := make(map[string]any, len(violations))

	for _, v := range violations {
		description := v.GetDescription()
		if key := meta[v.GetField()]; key != "" {
			description = localizer.Translate(key, description, nil)
		}
		fields[v.GetField()] = description
	}

	return server.
		UnprocessableEntity("validation_failed", "the request contains invalid fields").
		WithDetails(map[string]any{"fields": fields})
}

func extractDetails(st *status.Status) (string, map[string]string, []*errdetails.BadRequest_FieldViolation) {
	reason := ""
	meta := map[string]string{}
	var violations []*errdetails.BadRequest_FieldViolation

	for _, detail := range st.Details() {
		switch t := detail.(type) {
		case *errdetails.ErrorInfo:
			reason = t.GetReason()
			meta = t.GetMetadata()
		case *errdetails.BadRequest:
			violations = t.GetFieldViolations()
		}
	}

	return reason, meta, violations
}

func codeOr(code, fallback string) string {
	if code == "" {
		return fallback
	}
	return code
}
