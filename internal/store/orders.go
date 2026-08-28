package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/lakhans7/eventbasedhttp/internal/domain"
)

type OrderInput struct {
	FiverrAccountID string
	GigID           *string
	CustomerID      string
	ExternalRef     *string
	AmountCents     int64
	Currency        string
	Status          string
	Stage           string
	DueAt           *time.Time
	Source          string
}

const orderCols = `id, fiverr_account_id, gig_id, customer_id, external_ref, amount_cents, currency, status, stage, due_at, delivered_at, completed_at, cancelled_at, source, created_at, updated_at`

func scanOrder(row pgx.Row) (*domain.Order, error) {
	o := &domain.Order{}
	err := row.Scan(&o.ID, &o.FiverrAccountID, &o.GigID, &o.CustomerID, &o.ExternalRef, &o.AmountCents, &o.Currency,
		&o.Status, &o.Stage, &o.DueAt, &o.DeliveredAt, &o.CompletedAt, &o.CancelledAt, &o.Source, &o.CreatedAt, &o.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return o, err
}

func scanOrderRows(rows pgx.Rows) (*domain.Order, error) {
	o := &domain.Order{}
	err := rows.Scan(&o.ID, &o.FiverrAccountID, &o.GigID, &o.CustomerID, &o.ExternalRef, &o.AmountCents, &o.Currency,
		&o.Status, &o.Stage, &o.DueAt, &o.DeliveredAt, &o.CompletedAt, &o.CancelledAt, &o.Source, &o.CreatedAt, &o.UpdatedAt)
	return o, err
}

func (s *Store) CreateOrder(ctx context.Context, in OrderInput) (*domain.Order, error) {
	if in.Currency == "" {
		in.Currency = "usd"
	}
	if in.Status == "" {
		in.Status = domain.OrderStatusActive
	}
	if in.Stage == "" {
		in.Stage = domain.OrderStageCreated
	}
	if in.Source == "" {
		in.Source = domain.OrderSourceManual
	}
	row := s.Pool.QueryRow(ctx, `
		INSERT INTO orders (fiverr_account_id, gig_id, customer_id, external_ref, amount_cents, currency, status, stage, due_at, source)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		RETURNING `+orderCols, in.FiverrAccountID, in.GigID, in.CustomerID, in.ExternalRef, in.AmountCents, in.Currency, in.Status, in.Stage, in.DueAt, in.Source)
	return scanOrder(row)
}

type OrderFilter struct {
	FiverrAccountID string
	Status          string
	Limit           int
	Offset          int
}

func (s *Store) ListOrders(ctx context.Context, userID string, f OrderFilter) ([]domain.Order, error) {
	if f.Limit <= 0 || f.Limit > 200 {
		f.Limit = 50
	}
	rows, err := s.Pool.Query(ctx, `
		SELECT o.id, o.fiverr_account_id, o.gig_id, o.customer_id, o.external_ref, o.amount_cents, o.currency,
		       o.status, o.stage, o.due_at, o.delivered_at, o.completed_at, o.cancelled_at, o.source, o.created_at, o.updated_at
		FROM orders o JOIN fiverr_accounts fa ON fa.id = o.fiverr_account_id
		WHERE fa.user_id = $1 AND o.deleted_at IS NULL
		  AND ($2 = '' OR o.fiverr_account_id::text = $2)
		  AND ($3 = '' OR o.status = $3)
		ORDER BY o.due_at ASC NULLS LAST, o.created_at DESC
		LIMIT $4 OFFSET $5
	`, userID, f.FiverrAccountID, f.Status, f.Limit, f.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.Order
	for rows.Next() {
		o, err := scanOrderRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *o)
	}
	return out, rows.Err()
}

func (s *Store) GetOrder(ctx context.Context, id, userID string) (*domain.Order, error) {
	row := s.Pool.QueryRow(ctx, `
		SELECT o.id, o.fiverr_account_id, o.gig_id, o.customer_id, o.external_ref, o.amount_cents, o.currency,
		       o.status, o.stage, o.due_at, o.delivered_at, o.completed_at, o.cancelled_at, o.source, o.created_at, o.updated_at
		FROM orders o JOIN fiverr_accounts fa ON fa.id = o.fiverr_account_id
		WHERE o.id = $1 AND fa.user_id = $2 AND o.deleted_at IS NULL
	`, id, userID)
	return scanOrder(row)
}

type OrderPatch struct {
	Status *string
	Stage  *string
	DueAt  *time.Time
}

func (s *Store) PatchOrder(ctx context.Context, id, userID string, p OrderPatch) (*domain.Order, error) {
	row := s.Pool.QueryRow(ctx, `
		UPDATE orders o SET
			status = COALESCE($3, o.status),
			stage = COALESCE($4, o.stage),
			due_at = COALESCE($5, o.due_at),
			delivered_at = CASE WHEN $4 = 'delivered' THEN now() ELSE o.delivered_at END,
			completed_at = CASE WHEN $4 = 'completed' THEN now() ELSE o.completed_at END,
			cancelled_at = CASE WHEN $3 = 'cancelled' THEN now() ELSE o.cancelled_at END,
			updated_at = now()
		FROM fiverr_accounts fa
		WHERE o.id = $1 AND fa.id = o.fiverr_account_id AND fa.user_id = $2
		RETURNING o.id, o.fiverr_account_id, o.gig_id, o.customer_id, o.external_ref, o.amount_cents, o.currency,
		          o.status, o.stage, o.due_at, o.delivered_at, o.completed_at, o.cancelled_at, o.source, o.created_at, o.updated_at
	`, id, userID, p.Status, p.Stage, p.DueAt)
	return scanOrder(row)
}

// --- Order requirements ---

func (s *Store) CreateOrderRequirement(ctx context.Context, orderID, rawText string) (*domain.OrderRequirement, error) {
	r := &domain.OrderRequirement{}
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO order_requirements (order_id, raw_text) VALUES ($1, $2)
		RETURNING id, order_id, raw_text, created_at
	`, orderID, rawText).Scan(&r.ID, &r.OrderID, &r.RawText, &r.CreatedAt)
	return r, err
}

func (s *Store) GetOrderRequirement(ctx context.Context, id, userID string) (*domain.OrderRequirement, error) {
	r := &domain.OrderRequirement{}
	var extractedRaw []byte
	err := s.Pool.QueryRow(ctx, `
		SELECT oreq.id, oreq.order_id, oreq.raw_text, oreq.extracted, oreq.created_at
		FROM order_requirements oreq
		JOIN orders o ON o.id = oreq.order_id
		JOIN fiverr_accounts fa ON fa.id = o.fiverr_account_id
		WHERE oreq.id = $1 AND fa.user_id = $2
	`, id, userID).Scan(&r.ID, &r.OrderID, &r.RawText, &extractedRaw, &r.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if len(extractedRaw) > 0 {
		var ex domain.RequirementExtraction
		if json.Unmarshal(extractedRaw, &ex) == nil {
			r.Extracted = &ex
		}
	}
	return r, nil
}

func (s *Store) ListOrderRequirements(ctx context.Context, orderID string) ([]domain.OrderRequirement, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT id, order_id, raw_text, extracted, created_at FROM order_requirements WHERE order_id = $1 ORDER BY created_at
	`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.OrderRequirement
	for rows.Next() {
		var r domain.OrderRequirement
		var extractedRaw []byte
		if err := rows.Scan(&r.ID, &r.OrderID, &r.RawText, &extractedRaw, &r.CreatedAt); err != nil {
			return nil, err
		}
		if len(extractedRaw) > 0 {
			var ex domain.RequirementExtraction
			if json.Unmarshal(extractedRaw, &ex) == nil {
				r.Extracted = &ex
			}
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) SetRequirementExtraction(ctx context.Context, id string, extraction *domain.RequirementExtraction) error {
	b, err := json.Marshal(extraction)
	if err != nil {
		return err
	}
	_, err = s.Pool.Exec(ctx, `UPDATE order_requirements SET extracted = $2 WHERE id = $1`, id, b)
	return err
}
