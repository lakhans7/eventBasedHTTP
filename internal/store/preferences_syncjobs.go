package store

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/lakhans7/eventbasedhttp/internal/domain"
)

func (s *Store) GetSellerPreferences(ctx context.Context, userID string) (*domain.SellerPreferences, error) {
	p := &domain.SellerPreferences{UserID: userID}
	var faqsRaw []byte
	err := s.Pool.QueryRow(ctx, `
		SELECT skills, services, tone, min_project_usd, typical_delivery_days_min, typical_delivery_days_max, faqs, portfolio_links, COALESCE(terms,''), COALESCE(restrictions,'')
		FROM seller_preferences WHERE user_id = $1
	`, userID).Scan(&p.Skills, &p.Services, &p.Tone, &p.MinProjectUSD, &p.TypicalDeliveryDaysMin, &p.TypicalDeliveryDaysMax, &faqsRaw, &p.PortfolioLinks, &p.Terms, &p.Restrictions)
	if errors.Is(err, pgx.ErrNoRows) {
		// No preferences saved yet — return sane defaults without erroring, so a
		// brand-new seller's AI Assistant still has something reasonable to work with.
		return &domain.SellerPreferences{
			UserID:                 userID,
			Tone:                   "professional but friendly",
			TypicalDeliveryDaysMin: 3,
			TypicalDeliveryDaysMax: 7,
		}, nil
	}
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal(faqsRaw, &p.FAQs)
	return p, nil
}

func (s *Store) UpsertSellerPreferences(ctx context.Context, p *domain.SellerPreferences) error {
	faqs, err := json.Marshal(p.FAQs)
	if err != nil {
		return err
	}
	_, err = s.Pool.Exec(ctx, `
		INSERT INTO seller_preferences (user_id, skills, services, tone, min_project_usd, typical_delivery_days_min, typical_delivery_days_max, faqs, portfolio_links, terms, restrictions, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,NULLIF($10,''),NULLIF($11,''),now())
		ON CONFLICT (user_id) DO UPDATE SET
			skills = EXCLUDED.skills, services = EXCLUDED.services, tone = EXCLUDED.tone,
			min_project_usd = EXCLUDED.min_project_usd,
			typical_delivery_days_min = EXCLUDED.typical_delivery_days_min,
			typical_delivery_days_max = EXCLUDED.typical_delivery_days_max,
			faqs = EXCLUDED.faqs, portfolio_links = EXCLUDED.portfolio_links,
			terms = EXCLUDED.terms, restrictions = EXCLUDED.restrictions, updated_at = now()
	`, p.UserID, p.Skills, p.Services, p.Tone, p.MinProjectUSD, p.TypicalDeliveryDaysMin, p.TypicalDeliveryDaysMax, faqs, p.PortfolioLinks, p.Terms, p.Restrictions)
	return err
}

// --- Sync jobs ---

func (s *Store) CreateSyncJob(ctx context.Context, fiverrAccountID *string, jobType string) (*domain.SyncJob, error) {
	j := &domain.SyncJob{}
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO sync_jobs (fiverr_account_id, job_type, status) VALUES ($1, $2, $3)
		RETURNING id, fiverr_account_id, job_type, status, attempt, error, started_at, finished_at, created_at
	`, fiverrAccountID, jobType, domain.SyncJobStatusQueued).Scan(&j.ID, &j.FiverrAccountID, &j.JobType, &j.Status, &j.Attempt, &j.Error, &j.StartedAt, &j.FinishedAt, &j.CreatedAt)
	return j, err
}

func (s *Store) UpdateSyncJobStatus(ctx context.Context, id, status string, jobErr *string) error {
	_, err := s.Pool.Exec(ctx, `
		UPDATE sync_jobs SET status = $2, error = $3,
			attempt = attempt + (CASE WHEN $2 = 'running' THEN 1 ELSE 0 END),
			started_at = COALESCE(started_at, CASE WHEN $2 = 'running' THEN now() END),
			finished_at = CASE WHEN $2 IN ('succeeded','failed') THEN now() ELSE finished_at END
		WHERE id = $1
	`, id, status, jobErr)
	return err
}

func (s *Store) ListSyncJobs(ctx context.Context, fiverrAccountID, userID string) ([]domain.SyncJob, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT sj.id, sj.fiverr_account_id, sj.job_type, sj.status, sj.attempt, sj.error, sj.started_at, sj.finished_at, sj.created_at
		FROM sync_jobs sj JOIN fiverr_accounts fa ON fa.id = sj.fiverr_account_id
		WHERE sj.fiverr_account_id = $1 AND fa.user_id = $2
		ORDER BY sj.created_at DESC LIMIT 50
	`, fiverrAccountID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.SyncJob
	for rows.Next() {
		var j domain.SyncJob
		if err := rows.Scan(&j.ID, &j.FiverrAccountID, &j.JobType, &j.Status, &j.Attempt, &j.Error, &j.StartedAt, &j.FinishedAt, &j.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}
