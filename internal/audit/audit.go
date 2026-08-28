// Package audit records the audit trail required by docs/security.md and
// product section 18. Every write goes through Log, which redacts any
// metadata field whose name looks like a secret before it ever reaches the
// database or logs.
package audit

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/lakhans7/eventbasedhttp/internal/logger"
)

type Service struct {
	pool *pgxpool.Pool
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

type Entry struct {
	UserID       *string
	Action       string
	ResourceType string
	ResourceID   string
	Metadata     map[string]any
	IPAddress    string
	UserAgent    string
}

func (s *Service) Log(ctx context.Context, e Entry) {
	redacted := redact(e.Metadata)
	meta, err := json.Marshal(redacted)
	if err != nil {
		meta = []byte("{}")
	}

	_, err = s.pool.Exec(ctx, `
		INSERT INTO audit_logs (user_id, action, resource_type, resource_id, metadata, ip_address, user_agent)
		VALUES ($1, $2, NULLIF($3,''), NULLIF($4,''), $5, $6, $7)
	`, e.UserID, e.Action, e.ResourceType, e.ResourceID, meta, e.IPAddress, e.UserAgent)
	if err != nil {
		logger.Get().Error().Err(err).Str("action", e.Action).Msg("audit: failed to persist audit log entry")
	}
}

func redact(m map[string]any) map[string]any {
	if m == nil {
		return map[string]any{}
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		if logger.IsSecretField(k) {
			out[k] = "[redacted]"
			continue
		}
		out[k] = v
	}
	return out
}
