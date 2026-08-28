package api

import (
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/lakhans7/eventbasedhttp/internal/logger"
)

// RequestLogger emits one structured log line per request. It never logs
// request/response bodies (which could contain a password or buyer PII) —
// only method, path, status, latency, and request id.
func RequestLogger() fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()
		err := c.Next()
		logger.Get().Info().
			Str("method", c.Method()).
			Str("path", c.Path()).
			Int("status", c.Response().StatusCode()).
			Dur("latency", time.Since(start)).
			Str("request_id", c.GetRespHeader(fiber.HeaderXRequestID)).
			Msg("http_request")
		return err
	}
}
