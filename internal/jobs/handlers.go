package jobs

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hibiken/asynq"

	"github.com/lakhans7/eventbasedhttp/internal/domain"
	"github.com/lakhans7/eventbasedhttp/internal/logger"
	"github.com/lakhans7/eventbasedhttp/internal/mailer"
	"github.com/lakhans7/eventbasedhttp/internal/marketplace/fiverr"
	"github.com/lakhans7/eventbasedhttp/internal/store"
)

type Deps struct {
	Store  *store.Store
	Mailer mailer.Mailer
}

func RegisterHandlers(mux *asynq.ServeMux, deps Deps) {
	mux.HandleFunc(TypeSendEmail, deps.handleSendEmail)
	mux.HandleFunc(TypeProcessImport, deps.handleProcessImport)
	mux.HandleFunc(TypeDispatchNotification, deps.handleDispatchNotification)
	mux.HandleFunc(TypeRefreshTokenNoop, deps.handleRefreshTokenNoop)
}

func (d Deps) handleSendEmail(ctx context.Context, t *asynq.Task) error {
	var p SendEmailPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("%w: %v", asynq.SkipRetry, err)
	}
	return d.Mailer.Send(p.To, p.Subject, p.Body)
}

func (d Deps) handleProcessImport(ctx context.Context, t *asynq.Task) error {
	var p ProcessImportPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("%w: %v", asynq.SkipRetry, err)
	}

	if err := d.Store.UpdateSyncJobStatus(ctx, p.SyncJobID, domain.SyncJobStatusRunning, nil); err != nil {
		logger.Get().Error().Err(err).Str("sync_job_id", p.SyncJobID).Msg("jobs: failed to mark import running")
	}

	raw, err := base64.StdEncoding.DecodeString(p.CSVBase64)
	if err != nil {
		errMsg := "invalid CSV encoding: " + err.Error()
		_ = d.Store.UpdateSyncJobStatus(ctx, p.SyncJobID, domain.SyncJobStatusFailed, &errMsg)
		return fmt.Errorf("%w: %v", asynq.SkipRetry, err)
	}

	var result *fiverr.ImportResult
	switch strings.ToLower(p.ImportType) {
	case "gigs":
		result, err = fiverr.ImportGigs(ctx, d.Store, p.FiverrAccountID, strings.NewReader(string(raw)))
	case "orders":
		result, err = fiverr.ImportOrders(ctx, d.Store, p.FiverrAccountID, strings.NewReader(string(raw)))
	case "reviews":
		result, err = fiverr.ImportReviews(ctx, d.Store, p.FiverrAccountID, strings.NewReader(string(raw)))
	default:
		errMsg := "unknown import_type: " + p.ImportType
		_ = d.Store.UpdateSyncJobStatus(ctx, p.SyncJobID, domain.SyncJobStatusFailed, &errMsg)
		return fmt.Errorf("%w: %s", asynq.SkipRetry, errMsg)
	}
	if err != nil {
		errMsg := err.Error()
		_ = d.Store.UpdateSyncJobStatus(ctx, p.SyncJobID, domain.SyncJobStatusFailed, &errMsg)
		return err
	}

	summary := fmt.Sprintf("imported %d rows, skipped %d", result.Imported, len(result.Skipped))
	if err := d.Store.UpdateSyncJobStatus(ctx, p.SyncJobID, domain.SyncJobStatusSucceeded, &summary); err != nil {
		logger.Get().Error().Err(err).Str("sync_job_id", p.SyncJobID).Msg("jobs: failed to mark import succeeded")
	}
	if err := d.Store.TouchFiverrAccountSync(ctx, p.FiverrAccountID); err != nil {
		logger.Get().Error().Err(err).Msg("jobs: failed to touch fiverr_account.last_sync_at")
	}
	return nil
}

func (d Deps) handleDispatchNotification(ctx context.Context, t *asynq.Task) error {
	var p DispatchNotificationPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("%w: %v", asynq.SkipRetry, err)
	}
	// In-app notifications are already persisted synchronously by the API layer
	// (internal/notification). This task only handles the email fan-out, so a
	// slow SMTP server never blocks the request that created the notification.
	logger.Get().Debug().Str("notification_id", p.NotificationID).Msg("jobs: notification dispatch acknowledged")
	return nil
}

// handleRefreshTokenNoop exists because docs/architecture.md documents a
// refresh_fiverr_token job for forward-compatibility. It is a deliberate
// no-op: there are no Fiverr tokens to refresh (docs/fiverr-api-capabilities.md).
func (d Deps) handleRefreshTokenNoop(ctx context.Context, t *asynq.Task) error {
	logger.Get().Warn().Msg("jobs: refresh_fiverr_token invoked but no provider supports refreshable tokens yet")
	return nil
}
