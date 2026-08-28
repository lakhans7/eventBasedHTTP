// Package analytics computes every metric directly from whatever data has
// actually been imported/entered — never from data that doesn't exist.
// Every response is timestamped so a partially-imported account never shows
// a silently stale or misleading number (product section 14).
package analytics

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	pool *pgxpool.Pool
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

type Overview struct {
	RevenueTotalCents       int64     `json:"revenue_total_cents"`
	OrdersTotal             int       `json:"orders_total"`
	OrdersActive            int       `json:"orders_active"`
	OrdersCompleted         int       `json:"orders_completed"`
	OrdersCancelled         int       `json:"orders_cancelled"`
	AverageOrderValueCents  int64     `json:"average_order_value_cents"`
	CompletionRatePercent   float64   `json:"completion_rate_percent"`
	CancellationRatePercent float64   `json:"cancellation_rate_percent"`
	AverageRating           float64   `json:"average_rating"`
	ReviewCount             int       `json:"review_count"`
	RepeatBuyerCount        int       `json:"repeat_buyer_count"`
	AsOf                    time.Time `json:"as_of"`
	Note                    string    `json:"note"`
}

func (s *Service) Overview(ctx context.Context, userID string) (*Overview, error) {
	o := &Overview{AsOf: time.Now(), Note: "Computed only from data you have manually entered or imported — Fiverr has no live API, so this is not real-time (docs/fiverr-api-capabilities.md)."}

	err := s.pool.QueryRow(ctx, `
		SELECT
			COALESCE(SUM(o.amount_cents), 0),
			COUNT(*),
			COUNT(*) FILTER (WHERE o.status = 'active'),
			COUNT(*) FILTER (WHERE o.status = 'completed'),
			COUNT(*) FILTER (WHERE o.status = 'cancelled')
		FROM orders o
		JOIN fiverr_accounts fa ON fa.id = o.fiverr_account_id
		WHERE fa.user_id = $1 AND o.deleted_at IS NULL
	`, userID).Scan(&o.RevenueTotalCents, &o.OrdersTotal, &o.OrdersActive, &o.OrdersCompleted, &o.OrdersCancelled)
	if err != nil {
		return nil, err
	}

	if o.OrdersTotal > 0 {
		o.AverageOrderValueCents = o.RevenueTotalCents / int64(o.OrdersTotal)
		o.CompletionRatePercent = round2(100 * float64(o.OrdersCompleted) / float64(o.OrdersTotal))
		o.CancellationRatePercent = round2(100 * float64(o.OrdersCancelled) / float64(o.OrdersTotal))
	}

	err = s.pool.QueryRow(ctx, `
		SELECT COALESCE(AVG(r.rating), 0), COUNT(*)
		FROM reviews r JOIN fiverr_accounts fa ON fa.id = r.fiverr_account_id
		WHERE fa.user_id = $1
	`, userID).Scan(&o.AverageRating, &o.ReviewCount)
	if err != nil {
		return nil, err
	}
	o.AverageRating = round2(o.AverageRating)

	err = s.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM (
			SELECT o.customer_id FROM orders o JOIN fiverr_accounts fa ON fa.id = o.fiverr_account_id
			WHERE fa.user_id = $1 AND o.deleted_at IS NULL
			GROUP BY o.customer_id HAVING COUNT(*) > 1
		) repeat_buyers
	`, userID).Scan(&o.RepeatBuyerCount)
	if err != nil {
		return nil, err
	}

	return o, nil
}

func round2(f float64) float64 {
	return float64(int(f*100+0.5)) / 100
}

type TimeSeriesPoint struct {
	Date  string `json:"date"`
	Value int64  `json:"value"`
}

func (s *Service) RevenueOverTime(ctx context.Context, userID string) ([]TimeSeriesPoint, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT to_char(date_trunc('day', o.created_at), 'YYYY-MM-DD'), COALESCE(SUM(o.amount_cents), 0)
		FROM orders o JOIN fiverr_accounts fa ON fa.id = o.fiverr_account_id
		WHERE fa.user_id = $1 AND o.deleted_at IS NULL
		GROUP BY 1 ORDER BY 1
	`, userID)
	return scanTimeSeries(rows, err)
}

func (s *Service) OrdersOverTime(ctx context.Context, userID string) ([]TimeSeriesPoint, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT to_char(date_trunc('day', o.created_at), 'YYYY-MM-DD'), COUNT(*)
		FROM orders o JOIN fiverr_accounts fa ON fa.id = o.fiverr_account_id
		WHERE fa.user_id = $1 AND o.deleted_at IS NULL
		GROUP BY 1 ORDER BY 1
	`, userID)
	return scanTimeSeries(rows, err)
}

func scanTimeSeries(rows pgx.Rows, err error) ([]TimeSeriesPoint, error) {
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []TimeSeriesPoint
	for rows.Next() {
		var p TimeSeriesPoint
		if err := rows.Scan(&p.Date, &p.Value); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
