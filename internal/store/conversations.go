package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/lakhans7/eventbasedhttp/internal/domain"
)

func (s *Store) GetOrCreateConversation(ctx context.Context, fiverrAccountID, customerID string, gigID *string) (*domain.Conversation, error) {
	// Reuse an existing conversation with the same customer (and gig, when provided) rather than fragmenting history.
	row := s.Pool.QueryRow(ctx, `
		SELECT id, fiverr_account_id, customer_id, gig_id, last_message_at, unread_count, created_at
		FROM conversations
		WHERE fiverr_account_id = $1 AND customer_id = $2 AND (gig_id = $3 OR ($3 IS NULL AND gig_id IS NULL))
		LIMIT 1
	`, fiverrAccountID, customerID, gigID)
	c := &domain.Conversation{}
	err := row.Scan(&c.ID, &c.FiverrAccountID, &c.CustomerID, &c.GigID, &c.LastMessageAt, &c.UnreadCount, &c.CreatedAt)
	if err == nil {
		return c, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	err = s.Pool.QueryRow(ctx, `
		INSERT INTO conversations (fiverr_account_id, customer_id, gig_id) VALUES ($1, $2, $3)
		RETURNING id, fiverr_account_id, customer_id, gig_id, last_message_at, unread_count, created_at
	`, fiverrAccountID, customerID, gigID).Scan(&c.ID, &c.FiverrAccountID, &c.CustomerID, &c.GigID, &c.LastMessageAt, &c.UnreadCount, &c.CreatedAt)
	return c, err
}

func (s *Store) ListConversations(ctx context.Context, userID string) ([]domain.Conversation, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT c.id, c.fiverr_account_id, c.customer_id, c.gig_id, c.last_message_at, c.unread_count, c.created_at
		FROM conversations c JOIN fiverr_accounts fa ON fa.id = c.fiverr_account_id
		WHERE fa.user_id = $1
		ORDER BY c.last_message_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.Conversation
	for rows.Next() {
		var c domain.Conversation
		if err := rows.Scan(&c.ID, &c.FiverrAccountID, &c.CustomerID, &c.GigID, &c.LastMessageAt, &c.UnreadCount, &c.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) GetConversation(ctx context.Context, id, userID string) (*domain.Conversation, error) {
	c := &domain.Conversation{}
	err := s.Pool.QueryRow(ctx, `
		SELECT c.id, c.fiverr_account_id, c.customer_id, c.gig_id, c.last_message_at, c.unread_count, c.created_at
		FROM conversations c JOIN fiverr_accounts fa ON fa.id = c.fiverr_account_id
		WHERE c.id = $1 AND fa.user_id = $2
	`, id, userID).Scan(&c.ID, &c.FiverrAccountID, &c.CustomerID, &c.GigID, &c.LastMessageAt, &c.UnreadCount, &c.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return c, err
}

type MessageInput struct {
	ConversationID string
	Direction      string
	Body           string
	SentAt         time.Time
	Source         string
	AIGenerationID *string
}

func (s *Store) AddMessage(ctx context.Context, in MessageInput) (*domain.Message, error) {
	if in.SentAt.IsZero() {
		in.SentAt = time.Now()
	}
	if in.Source == "" {
		in.Source = domain.MessageSourceManualPaste
	}

	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	m := &domain.Message{}
	err = tx.QueryRow(ctx, `
		INSERT INTO messages (conversation_id, direction, body, sent_at, source, ai_generation_id)
		VALUES ($1,$2,$3,$4,$5,$6)
		RETURNING id, conversation_id, direction, body, sent_at, source, ai_generation_id, created_at
	`, in.ConversationID, in.Direction, in.Body, in.SentAt, in.Source, in.AIGenerationID,
	).Scan(&m.ID, &m.ConversationID, &m.Direction, &m.Body, &m.SentAt, &m.Source, &m.AIGenerationID, &m.CreatedAt)
	if err != nil {
		return nil, err
	}

	unreadDelta := 0
	if in.Direction == domain.MessageDirectionInbound {
		unreadDelta = 1
	}
	if _, err := tx.Exec(ctx, `
		UPDATE conversations SET last_message_at = $2, unread_count = unread_count + $3 WHERE id = $1
	`, in.ConversationID, in.SentAt, unreadDelta); err != nil {
		return nil, err
	}

	return m, tx.Commit(ctx)
}

func (s *Store) ListMessages(ctx context.Context, conversationID, userID string, limit, offset int) ([]domain.Message, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.Pool.Query(ctx, `
		SELECT m.id, m.conversation_id, m.direction, m.body, m.sent_at, m.source, m.ai_generation_id, m.created_at
		FROM messages m
		JOIN conversations c ON c.id = m.conversation_id
		JOIN fiverr_accounts fa ON fa.id = c.fiverr_account_id
		WHERE m.conversation_id = $1 AND fa.user_id = $2
		ORDER BY m.sent_at ASC
		LIMIT $3 OFFSET $4
	`, conversationID, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.Message
	for rows.Next() {
		var m domain.Message
		if err := rows.Scan(&m.ID, &m.ConversationID, &m.Direction, &m.Body, &m.SentAt, &m.Source, &m.AIGenerationID, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Store) MarkConversationRead(ctx context.Context, id, userID string) error {
	tag, err := s.Pool.Exec(ctx, `
		UPDATE conversations c SET unread_count = 0
		FROM fiverr_accounts fa
		WHERE c.id = $1 AND fa.id = c.fiverr_account_id AND fa.user_id = $2
	`, id, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
