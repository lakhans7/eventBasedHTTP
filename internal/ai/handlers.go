package ai

import (
	"errors"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/lakhans7/eventbasedhttp/internal/audit"
	"github.com/lakhans7/eventbasedhttp/internal/auth"
	"github.com/lakhans7/eventbasedhttp/internal/domain"
	"github.com/lakhans7/eventbasedhttp/internal/store"
)

type Handlers struct {
	svc   *Service
	store *store.Store
	audit *audit.Service
}

func NewHandlers(svc *Service, st *store.Store, auditSvc *audit.Service) *Handlers {
	return &Handlers{svc: svc, store: st, audit: auditSvc}
}

func errJSON(c *fiber.Ctx, status int, code, message string) error {
	return c.Status(status).JSON(fiber.Map{"error": fiber.Map{"code": code, "message": message}})
}

func (h *Handlers) respond(c *fiber.Ctx, gen *domain.AIGeneration, err error) error {
	if errors.Is(err, ErrDailyBudgetExceeded) {
		return errJSON(c, fiber.StatusTooManyRequests, "ai_budget_exceeded", err.Error())
	}
	if err == store.ErrNotFound {
		return errJSON(c, fiber.StatusNotFound, "not_found", "Referenced resource not found.")
	}
	if err != nil {
		return errJSON(c, fiber.StatusBadGateway, "ai_generation_failed", err.Error())
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"generation": gen})
}

func (h *Handlers) GenerateResponse(c *fiber.Ctx) error {
	var req struct {
		ConversationID string `json:"conversation_id"`
		Instruction    string `json:"instruction"`
	}
	if err := c.BodyParser(&req); err != nil || req.ConversationID == "" {
		return errJSON(c, fiber.StatusBadRequest, "invalid_body", "conversation_id is required.")
	}
	gen, err := h.svc.GenerateMessageReply(c.Context(), auth.UserIDFromContext(c), req.ConversationID, req.Instruction)
	return h.respond(c, gen, err)
}

func (h *Handlers) SummarizeOrder(c *fiber.Ctx) error {
	var req struct {
		OrderID string `json:"order_id"`
	}
	if err := c.BodyParser(&req); err != nil || req.OrderID == "" {
		return errJSON(c, fiber.StatusBadRequest, "invalid_body", "order_id is required.")
	}
	gen, err := h.svc.SummarizeOrder(c.Context(), auth.UserIDFromContext(c), req.OrderID)
	return h.respond(c, gen, err)
}

func (h *Handlers) ExtractRequirements(c *fiber.Ctx) error {
	var req struct {
		OrderRequirementID string `json:"order_requirement_id"`
		Text               string `json:"text"`
	}
	if err := c.BodyParser(&req); err != nil {
		return errJSON(c, fiber.StatusBadRequest, "invalid_body", "Could not parse request body.")
	}
	userID := auth.UserIDFromContext(c)
	rawText := req.Text
	if req.OrderRequirementID != "" {
		r, err := h.store.GetOrderRequirement(c.Context(), req.OrderRequirementID, userID)
		if err != nil {
			return errJSON(c, fiber.StatusNotFound, "not_found", "Requirement not found.")
		}
		rawText = r.RawText
	}
	if rawText == "" {
		return errJSON(c, fiber.StatusBadRequest, "invalid_body", "order_requirement_id or text is required.")
	}
	gen, err := h.svc.ExtractRequirements(c.Context(), userID, req.OrderRequirementID, rawText)
	return h.respond(c, gen, err)
}

func (h *Handlers) DeliveryMessage(c *fiber.Ctx) error {
	var req struct {
		OrderID string `json:"order_id"`
	}
	if err := c.BodyParser(&req); err != nil || req.OrderID == "" {
		return errJSON(c, fiber.StatusBadRequest, "invalid_body", "order_id is required.")
	}
	gen, err := h.svc.DeliveryMessage(c.Context(), auth.UserIDFromContext(c), req.OrderID)
	return h.respond(c, gen, err)
}

func (h *Handlers) AnalyzeReview(c *fiber.Ctx) error {
	var req struct {
		ReviewID string `json:"review_id"`
	}
	if err := c.BodyParser(&req); err != nil || req.ReviewID == "" {
		return errJSON(c, fiber.StatusBadRequest, "invalid_body", "review_id is required.")
	}
	gen, err := h.svc.AnalyzeReview(c.Context(), auth.UserIDFromContext(c), req.ReviewID)
	return h.respond(c, gen, err)
}

func (h *Handlers) Chat(c *fiber.Ctx) error {
	var req struct {
		Question string `json:"question"`
		Context  string `json:"context"`
	}
	if err := c.BodyParser(&req); err != nil || req.Question == "" {
		return errJSON(c, fiber.StatusBadRequest, "invalid_body", "question is required.")
	}
	gen, err := h.svc.Chat(c.Context(), auth.UserIDFromContext(c), req.Question, req.Context)
	return h.respond(c, gen, err)
}

func (h *Handlers) GetGeneration(c *fiber.Ctx) error {
	gen, err := h.store.GetAIGeneration(c.Context(), c.Params("id"), auth.UserIDFromContext(c))
	if err != nil {
		return errJSON(c, fiber.StatusNotFound, "not_found", "Generation not found.")
	}
	return c.JSON(fiber.Map{"generation": gen})
}

func (h *Handlers) PatchGeneration(c *fiber.Ctx) error {
	var req struct {
		EditedOutput string `json:"edited_output"`
	}
	if err := c.BodyParser(&req); err != nil || req.EditedOutput == "" {
		return errJSON(c, fiber.StatusBadRequest, "invalid_body", "edited_output is required.")
	}
	userID := auth.UserIDFromContext(c)
	gen, err := h.store.UpdateAIGenerationOutput(c.Context(), c.Params("id"), userID, req.EditedOutput)
	if err != nil {
		return errJSON(c, fiber.StatusNotFound, "not_found", "Generation not found.")
	}
	h.audit.Log(c.Context(), audit.Entry{UserID: &userID, Action: "ai.edited", ResourceType: "ai_generation", ResourceID: gen.ID, IPAddress: c.IP(), UserAgent: c.Get("User-Agent")})
	return c.JSON(fiber.Map{"generation": gen})
}

func (h *Handlers) Approve(c *fiber.Ctx) error {
	userID := auth.UserIDFromContext(c)
	gen, err := h.store.SetAIGenerationStatus(c.Context(), c.Params("id"), userID, domain.AIGenerationStatusApproved, &userID)
	if err != nil {
		return errJSON(c, fiber.StatusNotFound, "not_found", "Generation not found.")
	}
	h.audit.Log(c.Context(), audit.Entry{UserID: &userID, Action: "ai.approved", ResourceType: "ai_generation", ResourceID: gen.ID, IPAddress: c.IP(), UserAgent: c.Get("User-Agent")})
	return c.JSON(fiber.Map{
		"generation": gen,
		"note":       "Fiverr has no API to send this for you. Copy this text into Fiverr's own inbox, then call /ai/generations/:id/mark-sent so it's recorded in your conversation history.",
	})
}

func (h *Handlers) Reject(c *fiber.Ctx) error {
	userID := auth.UserIDFromContext(c)
	gen, err := h.store.SetAIGenerationStatus(c.Context(), c.Params("id"), userID, domain.AIGenerationStatusRejected, nil)
	if err != nil {
		return errJSON(c, fiber.StatusNotFound, "not_found", "Generation not found.")
	}
	h.audit.Log(c.Context(), audit.Entry{UserID: &userID, Action: "ai.rejected", ResourceType: "ai_generation", ResourceID: gen.ID, IPAddress: c.IP(), UserAgent: c.Get("User-Agent")})
	return c.JSON(fiber.Map{"generation": gen})
}

func (h *Handlers) MarkSent(c *fiber.Ctx) error {
	userID := auth.UserIDFromContext(c)
	gen, err := h.store.GetAIGeneration(c.Context(), c.Params("id"), userID)
	if err != nil {
		return errJSON(c, fiber.StatusNotFound, "not_found", "Generation not found.")
	}
	if gen.Status != domain.AIGenerationStatusApproved {
		return errJSON(c, fiber.StatusConflict, "not_approved", "Only an approved generation can be marked as sent.")
	}

	if convID, ok := gen.ContextRef["conversation_id"].(string); ok && convID != "" {
		genID := gen.ID
		if _, err := h.store.AddMessage(c.Context(), store.MessageInput{
			ConversationID: convID,
			Direction:      domain.MessageDirectionOutbound,
			Body:           gen.DraftOutput,
			SentAt:         time.Now(),
			Source:         domain.MessageSourceManualPaste,
			AIGenerationID: &genID,
		}); err != nil {
			return errJSON(c, fiber.StatusInternalServerError, "internal_error", "Could not record the sent message.")
		}
	}

	gen, err = h.store.SetAIGenerationStatus(c.Context(), gen.ID, userID, domain.AIGenerationStatusSentExternally, &userID)
	if err != nil {
		return errJSON(c, fiber.StatusInternalServerError, "internal_error", "Could not update generation status.")
	}
	h.audit.Log(c.Context(), audit.Entry{UserID: &userID, Action: "message.marked_sent", ResourceType: "ai_generation", ResourceID: gen.ID, IPAddress: c.IP(), UserAgent: c.Get("User-Agent")})
	return c.JSON(fiber.Map{"generation": gen})
}

func (h *Handlers) Feedback(c *fiber.Ctx) error {
	var req struct {
		Rating  int    `json:"rating"`
		Comment string `json:"comment"`
	}
	if err := c.BodyParser(&req); err != nil || (req.Rating != 1 && req.Rating != -1) {
		return errJSON(c, fiber.StatusBadRequest, "invalid_body", "rating must be 1 or -1.")
	}
	userID := auth.UserIDFromContext(c)
	if err := h.store.CreateAIFeedback(c.Context(), c.Params("id"), userID, req.Rating, req.Comment); err != nil {
		return errJSON(c, fiber.StatusInternalServerError, "internal_error", "Could not save feedback.")
	}
	return c.SendStatus(fiber.StatusCreated)
}
