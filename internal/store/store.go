// Package store holds the marketplace-agnostic persistence layer for every
// normalized domain model (docs/database.md). It has no knowledge of Fiverr
// specifically — data arrives here already normalized by
// internal/marketplace/fiverr, whatever its original source (manual entry
// or CSV import today; a live API in the future).
package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/lakhans7/eventbasedhttp/internal/domain"
)

var ErrNotFound = errors.New("not found")

type Store struct {
	Pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Store {
	return &Store{Pool: pool}
}

// --- Fiverr accounts ---

func (s *Store) CreateFiverrAccount(ctx context.Context, userID, username string) (*domain.FiverrAccount, error) {
	a := &domain.FiverrAccount{}
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO fiverr_accounts (user_id, username, connection_method, status)
		VALUES ($1, $2, 'manual', 'connected')
		RETURNING id, user_id, username, connection_method, status, connected_at, created_at, updated_at
	`, userID, username).Scan(&a.ID, &a.UserID, &a.Username, &a.ConnectionMethod, &a.Status, &a.ConnectedAt, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return a, nil
}

func (s *Store) ListFiverrAccounts(ctx context.Context, userID string) ([]domain.FiverrAccount, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT id, user_id, username, connection_method, status, connected_at, last_sync_at, created_at, updated_at
		FROM fiverr_accounts WHERE user_id = $1 AND deleted_at IS NULL ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.FiverrAccount
	for rows.Next() {
		var a domain.FiverrAccount
		if err := rows.Scan(&a.ID, &a.UserID, &a.Username, &a.ConnectionMethod, &a.Status, &a.ConnectedAt, &a.LastSyncAt, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// GetFiverrAccount fetches an account and verifies ownership in the same query,
// so a handler can never leak another user's account by guessing an id.
func (s *Store) GetFiverrAccount(ctx context.Context, id, userID string) (*domain.FiverrAccount, error) {
	a := &domain.FiverrAccount{}
	err := s.Pool.QueryRow(ctx, `
		SELECT id, user_id, username, connection_method, status, connected_at, last_sync_at, created_at, updated_at
		FROM fiverr_accounts WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL
	`, id, userID).Scan(&a.ID, &a.UserID, &a.Username, &a.ConnectionMethod, &a.Status, &a.ConnectedAt, &a.LastSyncAt, &a.CreatedAt, &a.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return a, nil
}

func (s *Store) DisconnectFiverrAccount(ctx context.Context, id, userID string) error {
	tag, err := s.Pool.Exec(ctx, `
		UPDATE fiverr_accounts SET status = 'disconnected', updated_at = now(), deleted_at = now()
		WHERE id = $1 AND user_id = $2
	`, id, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) TouchFiverrAccountSync(ctx context.Context, id string) error {
	_, err := s.Pool.Exec(ctx, `UPDATE fiverr_accounts SET last_sync_at = now(), updated_at = now() WHERE id = $1`, id)
	return err
}

// --- Customers ---

func (s *Store) GetOrCreateCustomer(ctx context.Context, fiverrAccountID, externalRef, displayName string) (*domain.Customer, error) {
	c := &domain.Customer{}
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO customers (fiverr_account_id, external_ref, display_name)
		VALUES ($1, $2, $3)
		ON CONFLICT (fiverr_account_id, external_ref)
		DO UPDATE SET display_name = COALESCE(NULLIF(EXCLUDED.display_name, ''), customers.display_name)
		RETURNING id, fiverr_account_id, external_ref, display_name, COALESCE(notes,''), created_at
	`, fiverrAccountID, externalRef, displayName).Scan(&c.ID, &c.FiverrAccountID, &c.ExternalRef, &c.DisplayName, &c.Notes, &c.CreatedAt)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (s *Store) ListCustomers(ctx context.Context, userID string) ([]domain.Customer, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT c.id, c.fiverr_account_id, c.external_ref, c.display_name, COALESCE(c.notes,''), c.created_at
		FROM customers c JOIN fiverr_accounts fa ON fa.id = c.fiverr_account_id
		WHERE fa.user_id = $1 ORDER BY c.created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.Customer
	for rows.Next() {
		var c domain.Customer
		if err := rows.Scan(&c.ID, &c.FiverrAccountID, &c.ExternalRef, &c.DisplayName, &c.Notes, &c.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
