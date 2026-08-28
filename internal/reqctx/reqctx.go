// Package reqctx holds the fiber.Ctx locals keys shared between the auth
// middleware (which sets them) and every other package that reads the
// authenticated caller's identity. It exists specifically so those readers
// (audit, ai, notification, api) don't need to import internal/auth just for
// this, which would create import cycles with packages internal/auth itself
// depends on (e.g. internal/audit).
package reqctx

import "github.com/gofiber/fiber/v2"

const (
	UserIDKey    = "auth_user_id"
	SessionIDKey = "auth_session_id"
)

func UserID(c *fiber.Ctx) string {
	v, _ := c.Locals(UserIDKey).(string)
	return v
}

func SessionID(c *fiber.Ctx) string {
	v, _ := c.Locals(SessionIDKey).(string)
	return v
}
