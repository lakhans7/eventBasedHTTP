package store

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/lakhans7/eventbasedhttp/internal/domain"
)

type GigInput struct {
	FiverrAccountID string
	ExternalRef     *string
	Title           string
	Description     string
	Category        string
	Status          string
	BasePriceCents  int64
	Currency        string
	Packages        []domain.GigPackage
	Metrics         *domain.GigMetrics
	Source          string
}

func (s *Store) UpsertGig(ctx context.Context, in GigInput) (*domain.Gig, error) {
	packagesJSON, err := json.Marshal(in.Packages)
	if err != nil {
		return nil, err
	}
	var metricsJSON []byte
	if in.Metrics != nil {
		metricsJSON, err = json.Marshal(in.Metrics)
		if err != nil {
			return nil, err
		}
	}
	if in.Currency == "" {
		in.Currency = "usd"
	}
	if in.Status == "" {
		in.Status = domain.GigStatusUnknown
	}
	if in.Source == "" {
		in.Source = domain.GigSourceManual
	}

	g := &domain.Gig{}
	var packagesRaw, metricsRaw []byte
	err = s.Pool.QueryRow(ctx, `
		INSERT INTO gigs (fiverr_account_id, external_ref, title, description, category, status, base_price_cents, currency, packages, metrics, source)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		RETURNING id, fiverr_account_id, external_ref, title, description, category, status, base_price_cents, currency, packages, metrics, source, created_at, updated_at
	`, in.FiverrAccountID, in.ExternalRef, in.Title, in.Description, in.Category, in.Status, in.BasePriceCents, in.Currency, packagesJSON, nullableJSON(metricsJSON), in.Source,
	).Scan(&g.ID, &g.FiverrAccountID, &g.ExternalRef, &g.Title, &g.Description, &g.Category, &g.Status, &g.BasePriceCents, &g.Currency, &packagesRaw, &metricsRaw, &g.Source, &g.CreatedAt, &g.UpdatedAt)
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal(packagesRaw, &g.Packages)
	if len(metricsRaw) > 0 {
		var m domain.GigMetrics
		if json.Unmarshal(metricsRaw, &m) == nil {
			g.Metrics = &m
		}
	}
	return g, nil
}

func nullableJSON(b []byte) any {
	if len(b) == 0 {
		return nil
	}
	return b
}

func scanGig(row pgx.Row) (*domain.Gig, error) {
	g := &domain.Gig{}
	var packagesRaw, metricsRaw []byte
	err := row.Scan(&g.ID, &g.FiverrAccountID, &g.ExternalRef, &g.Title, &g.Description, &g.Category, &g.Status, &g.BasePriceCents, &g.Currency, &packagesRaw, &metricsRaw, &g.Source, &g.CreatedAt, &g.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal(packagesRaw, &g.Packages)
	if len(metricsRaw) > 0 {
		var m domain.GigMetrics
		if json.Unmarshal(metricsRaw, &m) == nil {
			g.Metrics = &m
		}
	}
	return g, nil
}

func (s *Store) ListGigs(ctx context.Context, userID string, fiverrAccountID string) ([]domain.Gig, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT g.id, g.fiverr_account_id, g.external_ref, g.title, g.description, g.category, g.status,
		       g.base_price_cents, g.currency, g.packages, g.metrics, g.source, g.created_at, g.updated_at
		FROM gigs g JOIN fiverr_accounts fa ON fa.id = g.fiverr_account_id
		WHERE fa.user_id = $1 AND g.deleted_at IS NULL AND ($2 = '' OR g.fiverr_account_id::text = $2)
		ORDER BY g.created_at DESC
	`, userID, fiverrAccountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.Gig
	for rows.Next() {
		g, err := scanGigFromRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *g)
	}
	return out, rows.Err()
}

func scanGigFromRows(rows pgx.Rows) (*domain.Gig, error) {
	g := &domain.Gig{}
	var packagesRaw, metricsRaw []byte
	if err := rows.Scan(&g.ID, &g.FiverrAccountID, &g.ExternalRef, &g.Title, &g.Description, &g.Category, &g.Status, &g.BasePriceCents, &g.Currency, &packagesRaw, &metricsRaw, &g.Source, &g.CreatedAt, &g.UpdatedAt); err != nil {
		return nil, err
	}
	_ = json.Unmarshal(packagesRaw, &g.Packages)
	if len(metricsRaw) > 0 {
		var m domain.GigMetrics
		if json.Unmarshal(metricsRaw, &m) == nil {
			g.Metrics = &m
		}
	}
	return g, nil
}

func (s *Store) GetGig(ctx context.Context, id, userID string) (*domain.Gig, error) {
	row := s.Pool.QueryRow(ctx, `
		SELECT g.id, g.fiverr_account_id, g.external_ref, g.title, g.description, g.category, g.status,
		       g.base_price_cents, g.currency, g.packages, g.metrics, g.source, g.created_at, g.updated_at
		FROM gigs g JOIN fiverr_accounts fa ON fa.id = g.fiverr_account_id
		WHERE g.id = $1 AND fa.user_id = $2 AND g.deleted_at IS NULL
	`, id, userID)
	return scanGig(row)
}

type GigPatch struct {
	Title       *string
	Description *string
	Category    *string
	Status      *string
}

func (s *Store) PatchGig(ctx context.Context, id, userID string, p GigPatch) (*domain.Gig, error) {
	row := s.Pool.QueryRow(ctx, `
		UPDATE gigs g SET
			title = COALESCE($3, title),
			description = COALESCE($4, description),
			category = COALESCE($5, category),
			status = COALESCE($6, status),
			updated_at = now()
		FROM fiverr_accounts fa
		WHERE g.id = $1 AND fa.id = g.fiverr_account_id AND fa.user_id = $2
		RETURNING g.id, g.fiverr_account_id, g.external_ref, g.title, g.description, g.category, g.status,
		          g.base_price_cents, g.currency, g.packages, g.metrics, g.source, g.created_at, g.updated_at
	`, id, userID, p.Title, p.Description, p.Category, p.Status)
	return scanGig(row)
}
