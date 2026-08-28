package notification

import (
	"github.com/gofiber/fiber/v2"

	"github.com/lakhans7/eventbasedhttp/internal/auth"
	"github.com/lakhans7/eventbasedhttp/internal/store"
)

type Handlers struct {
	store *store.Store
}

func NewHandlers(st *store.Store) *Handlers {
	return &Handlers{store: st}
}

func (h *Handlers) List(c *fiber.Ctx) error {
	notifications, err := h.store.ListNotifications(c.Context(), auth.UserIDFromContext(c), 50)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": fiber.Map{"code": "internal_error", "message": "Could not list notifications."}})
	}
	return c.JSON(fiber.Map{"notifications": notifications})
}

func (h *Handlers) MarkRead(c *fiber.Ctx) error {
	if err := h.store.MarkNotificationRead(c.Context(), c.Params("id"), auth.UserIDFromContext(c)); err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": fiber.Map{"code": "not_found", "message": "Notification not found."}})
	}
	return c.JSON(fiber.Map{"status": "read"})
}
