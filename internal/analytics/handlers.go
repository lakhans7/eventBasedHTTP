package analytics

import (
	"github.com/gofiber/fiber/v2"

	"github.com/lakhans7/eventbasedhttp/internal/auth"
)

type Handlers struct {
	svc *Service
}

func NewHandlers(svc *Service) *Handlers {
	return &Handlers{svc: svc}
}

func errJSON(c *fiber.Ctx, status int, code, message string) error {
	return c.Status(status).JSON(fiber.Map{"error": fiber.Map{"code": code, "message": message}})
}

func (h *Handlers) Overview(c *fiber.Ctx) error {
	o, err := h.svc.Overview(c.Context(), auth.UserIDFromContext(c))
	if err != nil {
		return errJSON(c, fiber.StatusInternalServerError, "internal_error", "Could not compute analytics overview.")
	}
	return c.JSON(o)
}

func (h *Handlers) RevenueOverTime(c *fiber.Ctx) error {
	points, err := h.svc.RevenueOverTime(c.Context(), auth.UserIDFromContext(c))
	if err != nil {
		return errJSON(c, fiber.StatusInternalServerError, "internal_error", "Could not compute revenue over time.")
	}
	return c.JSON(fiber.Map{"points": points})
}

func (h *Handlers) OrdersOverTime(c *fiber.Ctx) error {
	points, err := h.svc.OrdersOverTime(c.Context(), auth.UserIDFromContext(c))
	if err != nil {
		return errJSON(c, fiber.StatusInternalServerError, "internal_error", "Could not compute orders over time.")
	}
	return c.JSON(fiber.Map{"points": points})
}
