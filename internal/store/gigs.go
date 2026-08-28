package store

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/lakhans7/eventbasedhttp/internal/domain"
)

// GigInput mirrors Fiverr's own gig-creation wizard sections (Overview /
// Pricing / Description & FAQ / Requirements) — see domain.Gig for why.
type GigInput struct {
	FiverrAccountID   string
	ExternalRef       *string
	Title             string
	Category          string
	SubCategory       string
	Tags              []string
	Status            string
	BasePriceCents    int64
	Currency          string
	Packages          []domain.GigPackage
	Description       string
	FAQs              []domain.FAQ
	BuyerRequirements string
	Metrics           *domain.GigMetrics
	Source            string
}

// gigCols (bare) is used in INSERT...RETURNING, where there's no table alias.
// gigSelectCols (g.-prefixed) is used in the joined SELECT queries below.
const gigCols = `id, fiverr_account_id, external_ref, title, category, sub_category, tags,
	status, base_price_cents, currency, packages, description, faqs, buyer_requirements,
	metrics, source, created_at, updated_at`

const gigSelectCols = `g.id, g.fiverr_account_id, g.external_ref, g.title, g.category, g.sub_category, g.tags,
	g.status, g.base_price_cents, g.currency, g.packages, g.description, g.faqs, g.buyer_requirements,
	g.metrics, g.source, g.created_at, g.updated_at`

// scanner is satisfied by both pgx.Row and pgx.Rows, so one scan function
// serves single-row and multi-row queries alike.
type scanner interface {
	Scan(dest ...any) error
}

func scanGig(s scanner) (*domain.Gig, error) {
	g := &domain.Gig{}
	var packagesRaw, faqsRaw, metricsRaw []byte
	err := s.Scan(&g.ID, &g.FiverrAccountID, &g.ExternalRef, &g.Title, &g.Category, &g.SubCategory, &g.Tags,
		&g.Status, &g.BasePriceCents, &g.Currency, &packagesRaw, &g.Description, &faqsRaw, &g.BuyerRequirements,
		&metricsRaw, &g.Source, &g.CreatedAt, &g.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal(packagesRaw, &g.Packages)
	_ = json.Unmarshal(faqsRaw, &g.FAQs)
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

// UpsertGig inserts a new gig row — used both for CSV import (source=csv_import)
// and manual creation via POST /gigs (source=manual).
func (s *Store) UpsertGig(ctx context.Context, in GigInput) (*domain.Gig, error) {
	packagesJSON, err := json.Marshal(in.Packages)
	if err != nil {
		return nil, err
	}
	faqsJSON, err := json.Marshal(in.FAQs)
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
	if in.Tags == nil {
		in.Tags = []string{}
	}

	row := s.Pool.QueryRow(ctx, `
		INSERT INTO gigs (fiverr_account_id, external_ref, title, category, sub_category, tags, status,
		                   base_price_cents, currency, packages, description, faqs, buyer_requirements, metrics, source)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
		RETURNING `+gigCols,
		in.FiverrAccountID, in.ExternalRef, in.Title, in.Category, in.SubCategory, in.Tags, in.Status,
		in.BasePriceCents, in.Currency, packagesJSON, in.Description, faqsJSON, in.BuyerRequirements,
		nullableJSON(metricsJSON), in.Source,
	)
	return scanGig(row)
}

func (s *Store) ListGigs(ctx context.Context, userID string, fiverrAccountID string) ([]domain.Gig, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT `+gigSelectCols+`
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
		g, err := scanGig(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *g)
	}
	return out, rows.Err()
}

func (s *Store) GetGig(ctx context.Context, id, userID string) (*domain.Gig, error) {
	row := s.Pool.QueryRow(ctx, `
		SELECT `+gigSelectCols+`
		FROM gigs g JOIN fiverr_accounts fa ON fa.id = g.fiverr_account_id
		WHERE g.id = $1 AND fa.user_id = $2 AND g.deleted_at IS NULL
	`, id, userID)
	return scanGig(row)
}

// GigPatch fields are pointers so a nil field leaves the column untouched
// (COALESCE) while an explicit empty slice/string clears it.
type GigPatch struct {
	Title             *string              `json:"title"`
	Category          *string              `json:"category"`
	SubCategory       *string              `json:"sub_category"`
	Tags              *[]string            `json:"tags"`
	Status            *string              `json:"status"`
	BasePriceCents    *int64               `json:"base_price_cents"`
	Currency          *string              `json:"currency"`
	Packages          *[]domain.GigPackage `json:"packages"`
	Description       *string              `json:"description"`
	FAQs              *[]domain.FAQ        `json:"faqs"`
	BuyerRequirements *string              `json:"buyer_requirements"`
}

func (s *Store) PatchGig(ctx context.Context, id, userID string, p GigPatch) (*domain.Gig, error) {
	var packagesJSON, faqsJSON any
	if p.Packages != nil {
		b, err := json.Marshal(*p.Packages)
		if err != nil {
			return nil, err
		}
		packagesJSON = b
	}
	if p.FAQs != nil {
		b, err := json.Marshal(*p.FAQs)
		if err != nil {
			return nil, err
		}
		faqsJSON = b
	}
	var tags any
	if p.Tags != nil {
		tags = *p.Tags
	}

	row := s.Pool.QueryRow(ctx, `
		UPDATE gigs g SET
			title = COALESCE($3, g.title),
			category = COALESCE($4, g.category),
			sub_category = COALESCE($5, g.sub_category),
			tags = COALESCE($6, g.tags),
			status = COALESCE($7, g.status),
			base_price_cents = COALESCE($8, g.base_price_cents),
			currency = COALESCE($9, g.currency),
			packages = COALESCE($10, g.packages),
			description = COALESCE($11, g.description),
			faqs = COALESCE($12, g.faqs),
			buyer_requirements = COALESCE($13, g.buyer_requirements),
			updated_at = now()
		FROM fiverr_accounts fa
		WHERE g.id = $1 AND fa.id = g.fiverr_account_id AND fa.user_id = $2
		RETURNING `+gigSelectCols,
		id, userID, p.Title, p.Category, p.SubCategory, tags, p.Status, p.BasePriceCents, p.Currency,
		packagesJSON, p.Description, faqsJSON, p.BuyerRequirements,
	)
	return scanGig(row)
}
