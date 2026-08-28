package store

import (
	"context"
	"time"

	"github.com/lakhans7/eventbasedhttp/internal/domain"
)

type ReviewInput struct {
	FiverrAccountID string
	GigID           *string
	OrderID         *string
	Rating          int
	Body            string
	PostedAt        time.Time
}

func (s *Store) CreateReview(ctx context.Context, in ReviewInput) (*domain.Review, error) {
	if in.PostedAt.IsZero() {
		in.PostedAt = time.Now()
	}
	r := &domain.Review{}
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO reviews (fiverr_account_id, gig_id, order_id, rating, body, posted_at)
		VALUES ($1,$2,$3,$4,$5,$6)
		RETURNING id, fiverr_account_id, gig_id, order_id, rating, COALESCE(body,''), sentiment, posted_at, created_at
	`, in.FiverrAccountID, in.GigID, in.OrderID, in.Rating, in.Body, in.PostedAt,
	).Scan(&r.ID, &r.FiverrAccountID, &r.GigID, &r.OrderID, &r.Rating, &r.Body, &r.Sentiment, &r.PostedAt, &r.CreatedAt)
	return r, err
}

func (s *Store) SetReviewSentiment(ctx context.Context, id, sentiment string) error {
	_, err := s.Pool.Exec(ctx, `UPDATE reviews SET sentiment = $2 WHERE id = $1`, id, sentiment)
	return err
}

func (s *Store) ListReviews(ctx context.Context, userID, fiverrAccountID string) ([]domain.Review, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT r.id, r.fiverr_account_id, r.gig_id, r.order_id, r.rating, COALESCE(r.body,''), r.sentiment, r.posted_at, r.created_at
		FROM reviews r JOIN fiverr_accounts fa ON fa.id = r.fiverr_account_id
		WHERE fa.user_id = $1 AND ($2 = '' OR r.fiverr_account_id::text = $2)
		ORDER BY r.posted_at DESC
	`, userID, fiverrAccountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.Review
	for rows.Next() {
		var r domain.Review
		if err := rows.Scan(&r.ID, &r.FiverrAccountID, &r.GigID, &r.OrderID, &r.Rating, &r.Body, &r.Sentiment, &r.PostedAt, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) GetReview(ctx context.Context, id, userID string) (*domain.Review, error) {
	r := &domain.Review{}
	err := s.Pool.QueryRow(ctx, `
		SELECT r.id, r.fiverr_account_id, r.gig_id, r.order_id, r.rating, COALESCE(r.body,''), r.sentiment, r.posted_at, r.created_at
		FROM reviews r JOIN fiverr_accounts fa ON fa.id = r.fiverr_account_id
		WHERE r.id = $1 AND fa.user_id = $2
	`, id, userID).Scan(&r.ID, &r.FiverrAccountID, &r.GigID, &r.OrderID, &r.Rating, &r.Body, &r.Sentiment, &r.PostedAt, &r.CreatedAt)
	return r, err
}

// --- Notifications ---

type NotificationInput struct {
	UserID       string
	Type         string
	Title        string
	Body         string
	ResourceType *string
	ResourceID   *string
	Channels     []string
}

func (s *Store) CreateNotification(ctx context.Context, in NotificationInput) (*domain.Notification, error) {
	if len(in.Channels) == 0 {
		in.Channels = []string{"in_app"}
	}
	n := &domain.Notification{}
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO notifications (user_id, type, title, body, resource_type, resource_id, channels)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		RETURNING id, user_id, type, title, body, resource_type, resource_id, read_at, channels, created_at
	`, in.UserID, in.Type, in.Title, in.Body, in.ResourceType, in.ResourceID, in.Channels,
	).Scan(&n.ID, &n.UserID, &n.Type, &n.Title, &n.Body, &n.ResourceType, &n.ResourceID, &n.ReadAt, &n.Channels, &n.CreatedAt)
	return n, err
}

func (s *Store) ListNotifications(ctx context.Context, userID string, limit int) ([]domain.Notification, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.Pool.Query(ctx, `
		SELECT id, user_id, type, title, body, resource_type, resource_id, read_at, channels, created_at
		FROM notifications WHERE user_id = $1 ORDER BY created_at DESC LIMIT $2
	`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.Notification
	for rows.Next() {
		var n domain.Notification
		if err := rows.Scan(&n.ID, &n.UserID, &n.Type, &n.Title, &n.Body, &n.ResourceType, &n.ResourceID, &n.ReadAt, &n.Channels, &n.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func (s *Store) MarkNotificationRead(ctx context.Context, id, userID string) error {
	tag, err := s.Pool.Exec(ctx, `UPDATE notifications SET read_at = now() WHERE id = $1 AND user_id = $2 AND read_at IS NULL`, id, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
