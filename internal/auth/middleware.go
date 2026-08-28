package auth

import (
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/lakhans7/eventbasedhttp/internal/reqctx"
)

// RequireAuth validates the Bearer access token and attaches the caller's
// user id / session id to the request context. It never trusts a
// client-supplied user id from the body or query string.
func RequireAuth(jwtIssuer *JWTIssuer) fiber.Handler {
	return func(c *fiber.Ctx) error {
		header := c.Get("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			return fiber.NewError(fiber.StatusUnauthorized, "missing bearer token")
		}
		token := strings.TrimPrefix(header, "Bearer ")

		claims, err := jwtIssuer.Parse(token)
		if err != nil {
			return fiber.NewError(fiber.StatusUnauthorized, "invalid or expired token")
		}

		c.Locals(reqctx.UserIDKey, claims.UserID)
		c.Locals(reqctx.SessionIDKey, claims.SessionID)
		return c.Next()
	}
}

// UserIDFromContext and SessionIDFromContext are thin, package-local-name
// wrappers kept so call sites written against internal/auth don't need to
// know about internal/reqctx. See internal/reqctx for why the underlying
// storage lives in its own tiny package.
func UserIDFromContext(c *fiber.Ctx) string    { return reqctx.UserID(c) }
func SessionIDFromContext(c *fiber.Ctx) string { return reqctx.SessionID(c) }
