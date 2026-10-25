package server

import (
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/requestid"
	"go.uber.org/zap"
)

// RequestLogger writes one access-log line per request. It resolves errors
// itself, because Fiber runs its error handler only after the chain unwinds;
// logging after c.Next() without handling the error would record a 200 for
// every failure.
func RequestLogger(log *zap.Logger) fiber.Handler {
	return func(c fiber.Ctx) error {
		start := time.Now()

		if chainErr := c.Next(); chainErr != nil {
			writeError(c, log, chainErr)
		}

		status := c.Response().StatusCode()
		fields := []zap.Field{
			zap.String("request_id", requestid.FromContext(c)),
			zap.String("method", c.Method()),
			zap.String("path", c.Path()),
			zap.Int("status", status),
			zap.Duration("duration", time.Since(start)),
			zap.String("ip", c.IP()),
		}

		switch {
		case status >= fiber.StatusInternalServerError:
			log.Error("http request", fields...)
		case status >= fiber.StatusBadRequest:
			log.Warn("http request", fields...)
		default:
			log.Info("http request", fields...)
		}

		return nil
	}
}
