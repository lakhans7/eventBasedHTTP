// Package jobs defines the asynq-backed background task queue (docs/architecture.md
// section G). Every task handler is idempotent (safe to run twice for the same
// payload) and relies on asynq's built-in exponential-backoff retry — nothing here
// polls Fiverr, because there is nothing to poll (docs/fiverr-api-capabilities.md).
package jobs

import (
	"encoding/json"

	"github.com/hibiken/asynq"
)

const (
	TypeSendEmail            = "email:send"
	TypeProcessImport        = "import:process"
	TypeDispatchNotification = "notification:dispatch"
	TypeRefreshTokenNoop     = "token:refresh_noop"
)

type SendEmailPayload struct {
	To      string `json:"to"`
	Subject string `json:"subject"`
	Body    string `json:"body"`
}

func NewSendEmailTask(p SendEmailPayload) (*asynq.Task, error) {
	b, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeSendEmail, b), nil
}

type ProcessImportPayload struct {
	SyncJobID       string `json:"sync_job_id"`
	FiverrAccountID string `json:"fiverr_account_id"`
	ImportType      string `json:"import_type"` // "gigs" | "orders" | "reviews"
	CSVBase64       string `json:"csv_base64"`
}

// NewProcessImportTask uses the sync job id as the asynq task ID, so
// re-enqueuing the same import (e.g. a client retry after a timeout) is a
// no-op deduplicated by asynq rather than double-importing the file.
func NewProcessImportTask(p ProcessImportPayload) (*asynq.Task, error) {
	b, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeProcessImport, b, asynq.TaskID("import-"+p.SyncJobID)), nil
}

type DispatchNotificationPayload struct {
	NotificationID string `json:"notification_id"`
}

func NewDispatchNotificationTask(p DispatchNotificationPayload) (*asynq.Task, error) {
	b, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeDispatchNotification, b), nil
}
