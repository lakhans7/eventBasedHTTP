// Package api wires the versioned REST router (docs/api.md) and holds
// handlers for the plain read/local-write resources (gigs, orders,
// customers, conversations, seller preferences) that don't need a home in a
// more specific package.
package api

import (
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/lakhans7/eventbasedhttp/internal/auth"
	"github.com/lakhans7/eventbasedhttp/internal/domain"
	"github.com/lakhans7/eventbasedhttp/internal/store"
)

type ResourceHandlers struct {
	store *store.Store
}

func NewResourceHandlers(st *store.Store) *ResourceHandlers {
	return &ResourceHandlers{store: st}
}

func errJSON(c *fiber.Ctx, status int, code, message string) error {
	return c.Status(status).JSON(fiber.Map{"error": fiber.Map{"code": code, "message": message}})
}

// --- Gigs ---

func (h *ResourceHandlers) ListGigs(c *fiber.Ctx) error {
	gigs, err := h.store.ListGigs(c.Context(), auth.UserIDFromContext(c), c.Query("fiverr_account_id"))
	if err != nil {
		return errJSON(c, fiber.StatusInternalServerError, "internal_error", "Could not list gigs.")
	}
	return c.JSON(fiber.Map{"gigs": gigs})
}

func (h *ResourceHandlers) GetGig(c *fiber.Ctx) error {
	gig, err := h.store.GetGig(c.Context(), c.Params("id"), auth.UserIDFromContext(c))
	if err != nil {
		return errJSON(c, fiber.StatusNotFound, "not_found", "Gig not found.")
	}
	return c.JSON(fiber.Map{"gig": gig})
}

func (h *ResourceHandlers) PatchGig(c *fiber.Ctx) error {
	var patch store.GigPatch
	if err := c.BodyParser(&patch); err != nil {
		return errJSON(c, fiber.StatusBadRequest, "invalid_body", "Could not parse request body.")
	}
	gig, err := h.store.PatchGig(c.Context(), c.Params("id"), auth.UserIDFromContext(c), patch)
	if err != nil {
		return errJSON(c, fiber.StatusNotFound, "not_found", "Gig not found.")
	}
	return c.JSON(fiber.Map{"gig": gig, "note": "This updates our local copy only — Fiverr has no gig write API (docs/fiverr-api-capabilities.md), so this does not change your live Fiverr gig."})
}

// --- Orders ---

func (h *ResourceHandlers) ListOrders(c *fiber.Ctx) error {
	limit := c.QueryInt("limit", 50)
	offset := c.QueryInt("offset", 0)
	orders, err := h.store.ListOrders(c.Context(), auth.UserIDFromContext(c), store.OrderFilter{
		FiverrAccountID: c.Query("fiverr_account_id"),
		Status:          c.Query("status"),
		Limit:           limit,
		Offset:          offset,
	})
	if err != nil {
		return errJSON(c, fiber.StatusInternalServerError, "internal_error", "Could not list orders.")
	}
	return c.JSON(fiber.Map{"orders": orders})
}

func (h *ResourceHandlers) GetOrder(c *fiber.Ctx) error {
	userID := auth.UserIDFromContext(c)
	order, err := h.store.GetOrder(c.Context(), c.Params("id"), userID)
	if err != nil {
		return errJSON(c, fiber.StatusNotFound, "not_found", "Order not found.")
	}
	requirements, err := h.store.ListOrderRequirements(c.Context(), order.ID)
	if err != nil {
		return errJSON(c, fiber.StatusInternalServerError, "internal_error", "Could not load order requirements.")
	}
	return c.JSON(fiber.Map{"order": order, "requirements": requirements})
}

type orderPatchRequest struct {
	Status *string    `json:"status"`
	Stage  *string    `json:"stage"`
	DueAt  *time.Time `json:"due_at"`
}

func (h *ResourceHandlers) PatchOrder(c *fiber.Ctx) error {
	var req orderPatchRequest
	if err := c.BodyParser(&req); err != nil {
		return errJSON(c, fiber.StatusBadRequest, "invalid_body", "Could not parse request body.")
	}
	order, err := h.store.PatchOrder(c.Context(), c.Params("id"), auth.UserIDFromContext(c), store.OrderPatch{
		Status: req.Status, Stage: req.Stage, DueAt: req.DueAt,
	})
	if err != nil {
		return errJSON(c, fiber.StatusNotFound, "not_found", "Order not found.")
	}
	return c.JSON(fiber.Map{"order": order})
}

func (h *ResourceHandlers) CreateOrderRequirement(c *fiber.Ctx) error {
	var req struct {
		RawText string `json:"raw_text"`
	}
	if err := c.BodyParser(&req); err != nil || req.RawText == "" {
		return errJSON(c, fiber.StatusBadRequest, "invalid_body", "raw_text is required.")
	}
	orderID := c.Params("id")
	if _, err := h.store.GetOrder(c.Context(), orderID, auth.UserIDFromContext(c)); err != nil {
		return errJSON(c, fiber.StatusNotFound, "not_found", "Order not found.")
	}
	req2, err := h.store.CreateOrderRequirement(c.Context(), orderID, req.RawText)
	if err != nil {
		return errJSON(c, fiber.StatusInternalServerError, "internal_error", "Could not save requirement.")
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"requirement": req2})
}

// --- Customers ---

func (h *ResourceHandlers) ListCustomers(c *fiber.Ctx) error {
	customers, err := h.store.ListCustomers(c.Context(), auth.UserIDFromContext(c))
	if err != nil {
		return errJSON(c, fiber.StatusInternalServerError, "internal_error", "Could not list customers.")
	}
	return c.JSON(fiber.Map{"customers": customers})
}

// --- Conversations ---

func (h *ResourceHandlers) ListConversations(c *fiber.Ctx) error {
	conversations, err := h.store.ListConversations(c.Context(), auth.UserIDFromContext(c))
	if err != nil {
		return errJSON(c, fiber.StatusInternalServerError, "internal_error", "Could not list conversations.")
	}
	return c.JSON(fiber.Map{"conversations": conversations})
}

func (h *ResourceHandlers) GetConversation(c *fiber.Ctx) error {
	conv, err := h.store.GetConversation(c.Context(), c.Params("id"), auth.UserIDFromContext(c))
	if err != nil {
		return errJSON(c, fiber.StatusNotFound, "not_found", "Conversation not found.")
	}
	return c.JSON(fiber.Map{"conversation": conv})
}

func (h *ResourceHandlers) ListMessages(c *fiber.Ctx) error {
	limit := c.QueryInt("limit", 50)
	offset := c.QueryInt("offset", 0)
	messages, err := h.store.ListMessages(c.Context(), c.Params("id"), auth.UserIDFromContext(c), limit, offset)
	if err != nil {
		return errJSON(c, fiber.StatusInternalServerError, "internal_error", "Could not list messages.")
	}
	return c.JSON(fiber.Map{"messages": messages})
}

func (h *ResourceHandlers) MarkConversationRead(c *fiber.Ctx) error {
	if err := h.store.MarkConversationRead(c.Context(), c.Params("id"), auth.UserIDFromContext(c)); err != nil {
		return errJSON(c, fiber.StatusNotFound, "not_found", "Conversation not found.")
	}
	return c.JSON(fiber.Map{"status": "read"})
}

// --- Reviews ---

func (h *ResourceHandlers) ListReviews(c *fiber.Ctx) error {
	reviews, err := h.store.ListReviews(c.Context(), auth.UserIDFromContext(c), c.Query("fiverr_account_id"))
	if err != nil {
		return errJSON(c, fiber.StatusInternalServerError, "internal_error", "Could not list reviews.")
	}
	return c.JSON(fiber.Map{"reviews": reviews})
}

// --- Seller preferences (AI knowledge base) ---

func (h *ResourceHandlers) GetPreferences(c *fiber.Ctx) error {
	prefs, err := h.store.GetSellerPreferences(c.Context(), auth.UserIDFromContext(c))
	if err != nil {
		return errJSON(c, fiber.StatusInternalServerError, "internal_error", "Could not load preferences.")
	}
	return c.JSON(fiber.Map{"preferences": prefs})
}

func (h *ResourceHandlers) PutPreferences(c *fiber.Ctx) error {
	var p domain.SellerPreferences
	if err := c.BodyParser(&p); err != nil {
		return errJSON(c, fiber.StatusBadRequest, "invalid_body", "Could not parse request body.")
	}
	p.UserID = auth.UserIDFromContext(c)
	if err := h.store.UpsertSellerPreferences(c.Context(), &p); err != nil {
		return errJSON(c, fiber.StatusInternalServerError, "internal_error", "Could not save preferences.")
	}
	return c.JSON(fiber.Map{"preferences": p})
}
