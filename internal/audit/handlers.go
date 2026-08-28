package audit

import (
	"context"
	"encoding/json"

	"github.com/gofiber/fiber/v2"

	"github.com/lakhans7/eventbasedhttp/internal/domain"
	"github.com/lakhans7/eventbasedhttp/internal/reqctx"
)

// ListForUser returns a user's own audit trail (GET /audit-logs is
// explicitly self-only per docs/api.md — there is no admin role yet).
func (s *Service) ListForUser(ctx context.Context, userID string, limit int) ([]domain.AuditLog, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, user_id, action, resource_type, resource_id, metadata, ip_address, user_agent, created_at
		FROM audit_logs WHERE user_id = $1 ORDER BY created_at DESC LIMIT $2
	`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.AuditLog
	for rows.Next() {
		var a domain.AuditLog
		var metaRaw []byte
		if err := rows.Scan(&a.ID, &a.UserID, &a.Action, &a.ResourceType, &a.ResourceID, &metaRaw, &a.IPAddress, &a.UserAgent, &a.CreatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(metaRaw, &a.Metadata)
		out = append(out, a)
	}
	return out, rows.Err()
}

type Handlers struct {
	svc *Service
}

func NewHandlers(svc *Service) *Handlers {
	return &Handlers{svc: svc}
}

func (h *Handlers) List(c *fiber.Ctx) error {
	logs, err := h.svc.ListForUser(c.Context(), reqctx.UserID(c), 50)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": fiber.Map{"code": "internal_error", "message": "Could not list audit logs."}})
	}
	return c.JSON(fiber.Map{"audit_logs": logs})
}
