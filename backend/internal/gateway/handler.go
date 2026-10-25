package gateway

import (
	"time"

	"github.com/gofiber/fiber/v3"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	userv1 "github.com/qvarkk/go-fiber-template/backend/gen/user/v1"
	"github.com/qvarkk/go-fiber-template/backend/internal/i18n"
	"github.com/qvarkk/go-fiber-template/backend/internal/server"
)

// Handler translates HTTP requests into users-service calls. It owns no
// business rules: validation lives in the service, so this only binds JSON,
// dials the client and maps the gRPC result onto the HTTP envelope.
type Handler struct {
	users userv1.UserServiceClient
}

func NewHandler(client *Client) *Handler {
	return &Handler{users: client.Users}
}

type createRequest struct {
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
}

type updateRequest struct {
	DisplayName string `json:"display_name"`
}

func (h *Handler) Create(c fiber.Ctx) error {
	var req createRequest
	if err := c.Bind().Body(&req); err != nil {
		return server.BadRequest("invalid_body", "the request body could not be parsed as JSON").Wrap(err)
	}

	resp, err := h.users.CreateUser(c.Context(), &userv1.CreateUserRequest{
		Email:       req.Email,
		DisplayName: req.DisplayName,
	})
	if err != nil {
		return fromGRPC(c, err)
	}

	return c.Status(fiber.StatusCreated).JSON(userJSON(resp.GetUser()))
}

func (h *Handler) List(c fiber.Ctx) error {
	limit := fiber.Query[int32](c, "limit", 0)
	offset := fiber.Query[int32](c, "offset", 0)

	resp, err := h.users.ListUsers(c.Context(), &userv1.ListUsersRequest{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return fromGRPC(c, err)
	}

	data := make([]map[string]any, 0, len(resp.GetUsers()))
	for _, u := range resp.GetUsers() {
		data = append(data, userJSON(u))
	}

	return c.JSON(fiber.Map{
		"data":   data,
		"limit":  resp.GetLimit(),
		"offset": resp.GetOffset(),
	})
}

func (h *Handler) Get(c fiber.Ctx) error {
	resp, err := h.users.GetUser(c.Context(), &userv1.GetUserRequest{Id: c.Params("id")})
	if err != nil {
		return fromGRPC(c, err)
	}
	return c.JSON(userJSON(resp.GetUser()))
}

func (h *Handler) Update(c fiber.Ctx) error {
	var req updateRequest
	if err := c.Bind().Body(&req); err != nil {
		return server.BadRequest("invalid_body", "the request body could not be parsed as JSON").Wrap(err)
	}

	resp, err := h.users.UpdateUser(c.Context(), &userv1.UpdateUserRequest{
		Id:          c.Params("id"),
		DisplayName: req.DisplayName,
	})
	if err != nil {
		return fromGRPC(c, err)
	}

	return c.JSON(userJSON(resp.GetUser()))
}

func (h *Handler) Delete(c fiber.Ctx) error {
	_, err := h.users.DeleteUser(c.Context(), &userv1.DeleteUserRequest{Id: c.Params("id")})
	if err != nil {
		return fromGRPC(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func userJSON(u *userv1.User) map[string]any {
	return map[string]any{
		"id":           u.GetId(),
		"email":        u.GetEmail(),
		"display_name": u.GetDisplayName(),
		"created_at":   formatTime(u.GetCreatedAt()),
		"updated_at":   formatTime(u.GetUpdatedAt()),
	}
}

func formatTime(t *timestamppb.Timestamp) string {
	if t == nil {
		return ""
	}
	return t.AsTime().UTC().Format(time.RFC3339Nano)
}

// fromGRPC maps a users-service status onto an APIError, so the error envelope
// and its i18n behave exactly as they did when the logic lived in this process.
func fromGRPC(c fiber.Ctx, err error) error {
	st, ok := status.FromError(err)
	if !ok {
		return server.Internal("upstream_error", "unexpected error from the users service").Wrap(err)
	}

	reason, meta, violations := extractDetails(st)

	switch st.Code() {
	case codes.InvalidArgument:
		if reason == "validation_failed" {
			return validationError(c, reason, meta, violations)
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

func validationError(c fiber.Ctx, reason string, meta map[string]string, violations []*errdetails.BadRequest_FieldViolation) error {
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

func extractDetails(st *status.Status) (reason string, meta map[string]string, violations []*errdetails.BadRequest_FieldViolation) {
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
