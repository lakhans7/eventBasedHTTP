package api

// Handlers for /fiverr/* (docs/api.md). Deliberately in package api rather
// than internal/marketplace/fiverr: these HTTP handlers need to enqueue
// background jobs (internal/jobs), and internal/jobs itself needs to call
// internal/marketplace/fiverr's CSV import functions — keeping the handlers
// here avoids a jobs<->fiverr import cycle while internal/marketplace/fiverr
// stays a pure, HTTP-agnostic package.

import (
	"encoding/base64"
	"io"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/hibiken/asynq"

	"github.com/lakhans7/eventbasedhttp/internal/audit"
	"github.com/lakhans7/eventbasedhttp/internal/auth"
	"github.com/lakhans7/eventbasedhttp/internal/domain"
	"github.com/lakhans7/eventbasedhttp/internal/jobs"
	"github.com/lakhans7/eventbasedhttp/internal/store"
)

// maxImportFileSize keeps CSV imports small enough to safely pass through
// the Redis-backed job queue as a base64 payload (see internal/jobs/tasks.go).
const maxImportFileSize = 2 << 20 // 2 MiB

type FiverrHandlers struct {
	store       *store.Store
	audit       *audit.Service
	asynqClient *asynq.Client
}

func NewFiverrHandlers(st *store.Store, auditSvc *audit.Service, asynqClient *asynq.Client) *FiverrHandlers {
	return &FiverrHandlers{store: st, audit: auditSvc, asynqClient: asynqClient}
}

// ListAccounts handles GET /fiverr/accounts.
func (h *FiverrHandlers) ListAccounts(c *fiber.Ctx) error {
	accounts, err := h.store.ListFiverrAccounts(c.Context(), auth.UserIDFromContext(c))
	if err != nil {
		return errJSON(c, fiber.StatusInternalServerError, "internal_error", "Could not list Fiverr accounts.")
	}
	return c.JSON(fiber.Map{"accounts": accounts})
}

// CreateAccount handles POST /fiverr/accounts. There is no OAuth flow to
// redirect to (docs/fiverr-api-capabilities.md) — this simply labels a
// manual data source with the seller's public Fiverr username.
func (h *FiverrHandlers) CreateAccount(c *fiber.Ctx) error {
	var req struct {
		Username string `json:"username"`
	}
	if err := c.BodyParser(&req); err != nil || strings.TrimSpace(req.Username) == "" {
		return errJSON(c, fiber.StatusBadRequest, "invalid_body", "A Fiverr username is required.")
	}
	userID := auth.UserIDFromContext(c)

	account, err := h.store.CreateFiverrAccount(c.Context(), userID, strings.TrimSpace(req.Username))
	if err != nil {
		return errJSON(c, fiber.StatusInternalServerError, "internal_error", "Could not create Fiverr account.")
	}
	h.audit.Log(c.Context(), audit.Entry{
		UserID: &userID, Action: "fiverr_account.connected", ResourceType: "fiverr_account", ResourceID: account.ID,
		Metadata:  map[string]any{"connection_method": "manual", "username": account.Username},
		IPAddress: c.IP(), UserAgent: c.Get("User-Agent"),
	})
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"account": account})
}

// DeleteAccount handles DELETE /fiverr/accounts/:id.
func (h *FiverrHandlers) DeleteAccount(c *fiber.Ctx) error {
	userID := auth.UserIDFromContext(c)
	id := c.Params("id")
	if err := h.store.DisconnectFiverrAccount(c.Context(), id, userID); err != nil {
		if err == store.ErrNotFound {
			return errJSON(c, fiber.StatusNotFound, "not_found", "Fiverr account not found.")
		}
		return errJSON(c, fiber.StatusInternalServerError, "internal_error", "Could not disconnect account.")
	}
	h.audit.Log(c.Context(), audit.Entry{UserID: &userID, Action: "fiverr_account.disconnected", ResourceType: "fiverr_account", ResourceID: id, IPAddress: c.IP(), UserAgent: c.Get("User-Agent")})
	return c.JSON(fiber.Map{"status": "disconnected"})
}

// Health handles GET /fiverr/accounts/:id/health.
func (h *FiverrHandlers) Health(c *fiber.Ctx) error {
	account, err := h.store.GetFiverrAccount(c.Context(), c.Params("id"), auth.UserIDFromContext(c))
	if err != nil {
		return errJSON(c, fiber.StatusNotFound, "not_found", "Fiverr account not found.")
	}
	return c.JSON(fiber.Map{
		"status":            account.Status,
		"connection_method": account.ConnectionMethod,
		"last_sync_at":      account.LastSyncAt,
		"note":              "Fiverr has no public API; data freshness depends on when you last imported or entered it manually.",
	})
}

// Import handles POST /fiverr/accounts/:id/import (multipart CSV upload).
func (h *FiverrHandlers) Import(c *fiber.Ctx) error {
	userID := auth.UserIDFromContext(c)
	accountID := c.Params("id")

	if _, err := h.store.GetFiverrAccount(c.Context(), accountID, userID); err != nil {
		return errJSON(c, fiber.StatusNotFound, "not_found", "Fiverr account not found.")
	}

	importType := strings.ToLower(strings.TrimSpace(c.FormValue("type")))
	if importType != "gigs" && importType != "orders" && importType != "reviews" {
		return errJSON(c, fiber.StatusBadRequest, "invalid_type", "type must be one of: gigs, orders, reviews.")
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		return errJSON(c, fiber.StatusBadRequest, "missing_file", "A CSV file is required (multipart field 'file').")
	}
	if fileHeader.Size > maxImportFileSize {
		return errJSON(c, fiber.StatusRequestEntityTooLarge, "file_too_large", "CSV files are limited to 2MB.")
	}

	f, err := fileHeader.Open()
	if err != nil {
		return errJSON(c, fiber.StatusBadRequest, "invalid_file", "Could not read uploaded file.")
	}
	defer f.Close()
	raw, err := io.ReadAll(f)
	if err != nil {
		return errJSON(c, fiber.StatusBadRequest, "invalid_file", "Could not read uploaded file.")
	}

	syncJob, err := h.store.CreateSyncJob(c.Context(), &accountID, "csv_import:"+importType)
	if err != nil {
		return errJSON(c, fiber.StatusInternalServerError, "internal_error", "Could not queue import.")
	}

	task, err := jobs.NewProcessImportTask(jobs.ProcessImportPayload{
		SyncJobID:       syncJob.ID,
		FiverrAccountID: accountID,
		ImportType:      importType,
		CSVBase64:       base64.StdEncoding.EncodeToString(raw),
	})
	if err != nil {
		return errJSON(c, fiber.StatusInternalServerError, "internal_error", "Could not queue import.")
	}
	if _, err := h.asynqClient.Enqueue(task); err != nil {
		return errJSON(c, fiber.StatusInternalServerError, "internal_error", "Could not queue import.")
	}

	h.audit.Log(c.Context(), audit.Entry{
		UserID: &userID, Action: "fiverr_account.import_queued", ResourceType: "fiverr_account", ResourceID: accountID,
		Metadata: map[string]any{"import_type": importType, "sync_job_id": syncJob.ID}, IPAddress: c.IP(), UserAgent: c.Get("User-Agent"),
	})
	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{"sync_job_id": syncJob.ID, "status": syncJob.Status})
}

// PostMessage handles POST /fiverr/accounts/:id/messages — a manual paste of
// a buyer/seller message, since there is no messaging API to read from.
func (h *FiverrHandlers) PostMessage(c *fiber.Ctx) error {
	userID := auth.UserIDFromContext(c)
	accountID := c.Params("id")

	if _, err := h.store.GetFiverrAccount(c.Context(), accountID, userID); err != nil {
		return errJSON(c, fiber.StatusNotFound, "not_found", "Fiverr account not found.")
	}

	var req struct {
		CustomerUsername string  `json:"customer_username"`
		GigID            *string `json:"gig_id"`
		Direction        string  `json:"direction"`
		Body             string  `json:"body"`
		SentAt           string  `json:"sent_at"`
	}
	if err := c.BodyParser(&req); err != nil {
		return errJSON(c, fiber.StatusBadRequest, "invalid_body", "Could not parse request body.")
	}
	if strings.TrimSpace(req.CustomerUsername) == "" || strings.TrimSpace(req.Body) == "" {
		return errJSON(c, fiber.StatusBadRequest, "invalid_body", "customer_username and body are required.")
	}
	if req.Direction != domain.MessageDirectionInbound && req.Direction != domain.MessageDirectionOutbound {
		return errJSON(c, fiber.StatusBadRequest, "invalid_direction", "direction must be 'inbound' or 'outbound'.")
	}

	sentAt := time.Now()
	if req.SentAt != "" {
		if t, err := time.Parse(time.RFC3339, req.SentAt); err == nil {
			sentAt = t
		}
	}

	customer, err := h.store.GetOrCreateCustomer(c.Context(), accountID, req.CustomerUsername, req.CustomerUsername)
	if err != nil {
		return errJSON(c, fiber.StatusInternalServerError, "internal_error", "Could not resolve customer.")
	}
	conversation, err := h.store.GetOrCreateConversation(c.Context(), accountID, customer.ID, req.GigID)
	if err != nil {
		return errJSON(c, fiber.StatusInternalServerError, "internal_error", "Could not resolve conversation.")
	}
	message, err := h.store.AddMessage(c.Context(), store.MessageInput{
		ConversationID: conversation.ID,
		Direction:      req.Direction,
		Body:           req.Body,
		SentAt:         sentAt,
		Source:         domain.MessageSourceManualPaste,
	})
	if err != nil {
		return errJSON(c, fiber.StatusInternalServerError, "internal_error", "Could not save message.")
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"message": message, "conversation_id": conversation.ID})
}

// ListSyncJobs handles GET /fiverr/accounts/:id/sync-jobs.
func (h *FiverrHandlers) ListSyncJobs(c *fiber.Ctx) error {
	userID := auth.UserIDFromContext(c)
	accountID := c.Params("id")
	syncJobs, err := h.store.ListSyncJobs(c.Context(), accountID, userID)
	if err != nil {
		return errJSON(c, fiber.StatusInternalServerError, "internal_error", "Could not list sync jobs.")
	}
	return c.JSON(fiber.Map{"sync_jobs": syncJobs})
}
