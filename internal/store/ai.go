package store

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/lakhans7/eventbasedhttp/internal/domain"
)

type AIGenerationInput struct {
	UserID           string
	Kind             string
	ContextRef       map[string]any
	Model            string
	InputTokens      int
	OutputTokens     int
	EstimatedCostUSD float64
	PromptRedacted   string
	DraftOutput      string
	RiskFlags        []string
}

func (s *Store) CreateAIGeneration(ctx context.Context, in AIGenerationInput) (*domain.AIGeneration, error) {
	ctxRef, err := json.Marshal(in.ContextRef)
	if err != nil {
		return nil, err
	}
	g := &domain.AIGeneration{}
	var ctxRaw []byte
	err = s.Pool.QueryRow(ctx, `
		INSERT INTO ai_generations (user_id, kind, context_ref, model, input_tokens, output_tokens, estimated_cost_usd, prompt_redacted, draft_output, risk_flags, status)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		RETURNING id, user_id, kind, context_ref, model, input_tokens, output_tokens, estimated_cost_usd, draft_output, risk_flags, status, approved_by, approved_at, created_at, updated_at
	`, in.UserID, in.Kind, ctxRef, in.Model, in.InputTokens, in.OutputTokens, in.EstimatedCostUSD, in.PromptRedacted, in.DraftOutput, in.RiskFlags, domain.AIGenerationStatusPendingReview,
	).Scan(&g.ID, &g.UserID, &g.Kind, &ctxRaw, &g.Model, &g.InputTokens, &g.OutputTokens, &g.EstimatedCostUSD, &g.DraftOutput, &g.RiskFlags, &g.Status, &g.ApprovedBy, &g.ApprovedAt, &g.CreatedAt, &g.UpdatedAt)
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal(ctxRaw, &g.ContextRef)
	return g, nil
}

func scanAIGeneration(row pgx.Row) (*domain.AIGeneration, error) {
	g := &domain.AIGeneration{}
	var ctxRaw []byte
	err := row.Scan(&g.ID, &g.UserID, &g.Kind, &ctxRaw, &g.Model, &g.InputTokens, &g.OutputTokens, &g.EstimatedCostUSD, &g.DraftOutput, &g.RiskFlags, &g.Status, &g.ApprovedBy, &g.ApprovedAt, &g.CreatedAt, &g.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal(ctxRaw, &g.ContextRef)
	return g, nil
}

func (s *Store) GetAIGeneration(ctx context.Context, id, userID string) (*domain.AIGeneration, error) {
	row := s.Pool.QueryRow(ctx, `
		SELECT id, user_id, kind, context_ref, model, input_tokens, output_tokens, estimated_cost_usd, draft_output, risk_flags, status, approved_by, approved_at, created_at, updated_at
		FROM ai_generations WHERE id = $1 AND user_id = $2
	`, id, userID)
	return scanAIGeneration(row)
}

func (s *Store) UpdateAIGenerationOutput(ctx context.Context, id, userID, editedOutput string) (*domain.AIGeneration, error) {
	row := s.Pool.QueryRow(ctx, `
		UPDATE ai_generations SET draft_output = $3, status = $4, updated_at = now()
		WHERE id = $1 AND user_id = $2
		RETURNING id, user_id, kind, context_ref, model, input_tokens, output_tokens, estimated_cost_usd, draft_output, risk_flags, status, approved_by, approved_at, created_at, updated_at
	`, id, userID, editedOutput, domain.AIGenerationStatusEdited)
	return scanAIGeneration(row)
}

func (s *Store) SetAIGenerationStatus(ctx context.Context, id, userID, status string, approvedBy *string) (*domain.AIGeneration, error) {
	row := s.Pool.QueryRow(ctx, `
		UPDATE ai_generations SET status = $3, approved_by = $4,
			approved_at = CASE WHEN $3 = 'approved' THEN now() ELSE approved_at END,
			updated_at = now()
		WHERE id = $1 AND user_id = $2
		RETURNING id, user_id, kind, context_ref, model, input_tokens, output_tokens, estimated_cost_usd, draft_output, risk_flags, status, approved_by, approved_at, created_at, updated_at
	`, id, userID, status, approvedBy)
	return scanAIGeneration(row)
}

func (s *Store) CreateAIFeedback(ctx context.Context, generationID, userID string, rating int, comment string) error {
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO ai_feedback (ai_generation_id, user_id, rating, comment) VALUES ($1,$2,$3,NULLIF($4,''))
	`, generationID, userID, rating, comment)
	return err
}

// SumAIUsageTodayUSD supports the per-user daily AI cost cap (docs/security.md, "Cost Control").
func (s *Store) SumAIUsageTodayUSD(ctx context.Context, userID string) (float64, error) {
	var total float64
	err := s.Pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(estimated_cost_usd), 0) FROM ai_generations
		WHERE user_id = $1 AND created_at >= date_trunc('day', now())
	`, userID).Scan(&total)
	return total, err
}
