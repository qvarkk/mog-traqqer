package server

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/requestid"
	"go.uber.org/zap"

	"github.com/qvarkk/go-fiber-template/backend/internal/i18n"
)

// ErrorBody describes a failure. Code is stable and never translated; Message
// is the human text, translated when the request has a language.
type ErrorBody struct {
	Code      string         `json:"code"`
	Message   string         `json:"message"`
	RequestID string         `json:"request_id,omitempty"`
	Details   map[string]any `json:"details,omitempty"`
}

type ErrorResponse struct {
	Error ErrorBody `json:"error"`
}

type APIError struct {
	Status  int
	Code    string
	Message string
	Details map[string]any
	Err     error
}

func (e *APIError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *APIError) Unwrap() error { return e.Err }

func (e *APIError) Wrap(err error) *APIError {
	clone := *e
	clone.Err = err
	return &clone
}

func (e *APIError) WithDetails(details map[string]any) *APIError {
	clone := *e
	clone.Details = details
	return &clone
}

func NewAPIError(status int, code, message string) *APIError {
	return &APIError{Status: status, Code: code, Message: message}
}

func BadRequest(code, message string) *APIError {
	return NewAPIError(http.StatusBadRequest, code, message)
}

func Unauthorized(code, message string) *APIError {
	return NewAPIError(http.StatusUnauthorized, code, message)
}

func Forbidden(code, message string) *APIError {
	return NewAPIError(http.StatusForbidden, code, message)
}

func NotFound(code, message string) *APIError {
	return NewAPIError(http.StatusNotFound, code, message)
}

func Conflict(code, message string) *APIError {
	return NewAPIError(http.StatusConflict, code, message)
}

func UnprocessableEntity(code, message string) *APIError {
	return NewAPIError(http.StatusUnprocessableEntity, code, message)
}

func Internal(code, message string) *APIError {
	return NewAPIError(http.StatusInternalServerError, code, message)
}

// errorHandler is the safety net for errors that never pass through
// RequestLogger, such as a panic in another middleware.
func errorHandler(log *zap.Logger) fiber.ErrorHandler {
	return func(c fiber.Ctx, err error) error {
		writeError(c, log, err)
		return nil
	}
}

// writeError is the single place where errors become HTTP responses.
func writeError(c fiber.Ctx, log *zap.Logger, err error) {
	requestID := requestid.FromContext(c)

	status := http.StatusInternalServerError
	code := defaultErrorCode(status)
	message := http.StatusText(status)
	cause := err

	var details map[string]any
	var apiErr *APIError
	var fiberErr *fiber.Error

	switch {
	case errors.As(err, &apiErr):
		status = apiErr.Status
		code = apiErr.Code
		cause = apiErr.Err
		details = apiErr.Details
		if apiErr.Message != "" {
			message = apiErr.Message
		}
	case errors.As(err, &fiberErr):
		status = fiberErr.Code
		code = defaultErrorCode(status)
		message = fiberErr.Message
	}

	// A 5xx is ours: log the cause, send a generic message, drop details.
	if status >= http.StatusInternalServerError {
		message = http.StatusText(status)
		if message == "" {
			message = http.StatusText(http.StatusInternalServerError)
		}
		details = nil

		log.Error("request failed",
			zap.String("request_id", requestID),
			zap.Int("status", status),
			zap.String("code", code),
			zap.Error(cause),
		)
	}

	// The stable code doubles as the translation key, so any error becomes
	// translatable by adding "error.<code>" to a catalog.
	message = i18n.FromContext(c).Translate("error."+code, message, nil)

	if err := c.Status(status).JSON(ErrorResponse{Error: ErrorBody{
		Code:      code,
		Message:   message,
		RequestID: requestID,
		Details:   details,
	}}); err != nil {
		log.Error("failed to write error response",
			zap.String("request_id", requestID),
			zap.Error(err),
		)
	}
}

func defaultErrorCode(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "bad_request"
	case http.StatusUnauthorized:
		return "unauthorized"
	case http.StatusForbidden:
		return "forbidden"
	case http.StatusNotFound:
		return "not_found"
	case http.StatusMethodNotAllowed:
		return "method_not_allowed"
	case http.StatusConflict:
		return "conflict"
	case http.StatusRequestEntityTooLarge:
		return "payload_too_large"
	case http.StatusUnprocessableEntity:
		return "unprocessable_entity"
	case http.StatusTooManyRequests:
		return "too_many_requests"
	case http.StatusInternalServerError:
		return "internal_error"
	default:
		if status >= http.StatusInternalServerError {
			return "internal_error"
		}
		return "request_failed"
	}
}
