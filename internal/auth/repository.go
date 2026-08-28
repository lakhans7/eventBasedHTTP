package auth

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/lakhans7/eventbasedhttp/internal/domain"
)

var ErrNotFound = errors.New("not found")

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) CreateUser(ctx context.Context, email, name, passwordHash string) (*domain.User, error) {
	u := &domain.User{}
	err := r.pool.QueryRow(ctx, `
		INSERT INTO users (email, name, password_hash, status)
		VALUES ($1, $2, NULLIF($3, ''), $4)
		RETURNING id, email, name, status, created_at, updated_at
	`, email, name, passwordHash, domain.UserStatusActive).Scan(&u.ID, &u.Email, &u.Name, &u.Status, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func (r *Repository) GetUserByEmail(ctx context.Context, email string) (*domain.User, error) {
	u := &domain.User{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, email, name, COALESCE(password_hash, ''), COALESCE(avatar_url, ''), status,
		       email_verified_at, last_login_at, created_at, updated_at
		FROM users WHERE email = $1 AND deleted_at IS NULL
	`, email).Scan(&u.ID, &u.Email, &u.Name, &u.PasswordHash, &u.AvatarURL, &u.Status,
		&u.EmailVerifiedAt, &u.LastLoginAt, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return u, nil
}

func (r *Repository) GetUserByID(ctx context.Context, id string) (*domain.User, error) {
	u := &domain.User{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, email, name, COALESCE(password_hash, ''), COALESCE(avatar_url, ''), status,
		       email_verified_at, last_login_at, created_at, updated_at
		FROM users WHERE id = $1 AND deleted_at IS NULL
	`, id).Scan(&u.ID, &u.Email, &u.Name, &u.PasswordHash, &u.AvatarURL, &u.Status,
		&u.EmailVerifiedAt, &u.LastLoginAt, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return u, nil
}

func (r *Repository) TouchLastLogin(ctx context.Context, userID string) error {
	_, err := r.pool.Exec(ctx, `UPDATE users SET last_login_at = now(), updated_at = now() WHERE id = $1`, userID)
	return err
}

func (r *Repository) MarkEmailVerified(ctx context.Context, userID string) error {
	_, err := r.pool.Exec(ctx, `UPDATE users SET email_verified_at = now(), updated_at = now() WHERE id = $1`, userID)
	return err
}

func (r *Repository) UpdatePassword(ctx context.Context, userID, passwordHash string) error {
	_, err := r.pool.Exec(ctx, `UPDATE users SET password_hash = $2, updated_at = now() WHERE id = $1`, userID, passwordHash)
	return err
}

func (r *Repository) SoftDeleteUser(ctx context.Context, userID string) error {
	_, err := r.pool.Exec(ctx, `UPDATE users SET status = $2, deleted_at = now(), updated_at = now() WHERE id = $1`,
		userID, domain.UserStatusPendingDeletion)
	return err
}

// --- Google identity linking ---

func (r *Repository) GetUserByGoogleID(ctx context.Context, googleUserID string) (*domain.User, error) {
	u := &domain.User{}
	err := r.pool.QueryRow(ctx, `
		SELECT u.id, u.email, u.name, COALESCE(u.avatar_url,''), u.status, u.created_at, u.updated_at
		FROM users u JOIN auth_identities ai ON ai.user_id = u.id
		WHERE ai.provider = 'google' AND ai.provider_user_id = $1 AND u.deleted_at IS NULL
	`, googleUserID).Scan(&u.ID, &u.Email, &u.Name, &u.AvatarURL, &u.Status, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return u, nil
}

func (r *Repository) LinkGoogleIdentity(ctx context.Context, userID, googleUserID string) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO auth_identities (user_id, provider, provider_user_id)
		VALUES ($1, 'google', $2) ON CONFLICT (provider, provider_user_id) DO NOTHING
	`, userID, googleUserID)
	return err
}

func (r *Repository) CreateUserFromGoogle(ctx context.Context, email, name, avatarURL string) (*domain.User, error) {
	u := &domain.User{}
	err := r.pool.QueryRow(ctx, `
		INSERT INTO users (email, name, avatar_url, status, email_verified_at)
		VALUES ($1, $2, $3, $4, now())
		RETURNING id, email, name, status, created_at, updated_at
	`, email, name, avatarURL, domain.UserStatusActive).Scan(&u.ID, &u.Email, &u.Name, &u.Status, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return u, nil
}

// --- Sessions (refresh tokens) ---

type Session struct {
	ID        string
	UserID    string
	ExpiresAt time.Time
	RevokedAt *time.Time
}

func (r *Repository) CreateSession(ctx context.Context, userID, refreshTokenHash, userAgent, ip string, expiresAt time.Time) (string, error) {
	var id string
	err := r.pool.QueryRow(ctx, `
		INSERT INTO sessions (user_id, refresh_token_hash, user_agent, ip_address, expires_at)
		VALUES ($1, $2, $3, $4, $5) RETURNING id
	`, userID, refreshTokenHash, userAgent, ip, expiresAt).Scan(&id)
	return id, err
}

func (r *Repository) GetSessionByRefreshHash(ctx context.Context, refreshTokenHash string) (*Session, error) {
	s := &Session{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, user_id, expires_at, revoked_at FROM sessions WHERE refresh_token_hash = $1
	`, refreshTokenHash).Scan(&s.ID, &s.UserID, &s.ExpiresAt, &s.RevokedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return s, nil
}

func (r *Repository) RevokeSession(ctx context.Context, sessionID string) error {
	_, err := r.pool.Exec(ctx, `UPDATE sessions SET revoked_at = now() WHERE id = $1 AND revoked_at IS NULL`, sessionID)
	return err
}

func (r *Repository) RevokeAllSessionsForUser(ctx context.Context, userID string) error {
	_, err := r.pool.Exec(ctx, `UPDATE sessions SET revoked_at = now() WHERE user_id = $1 AND revoked_at IS NULL`, userID)
	return err
}

func (r *Repository) ReplaceSession(ctx context.Context, oldSessionID, userID, newRefreshTokenHash, userAgent, ip string, expiresAt time.Time) (string, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `UPDATE sessions SET revoked_at = now() WHERE id = $1`, oldSessionID); err != nil {
		return "", err
	}
	var id string
	if err := tx.QueryRow(ctx, `
		INSERT INTO sessions (user_id, refresh_token_hash, user_agent, ip_address, expires_at)
		VALUES ($1, $2, $3, $4, $5) RETURNING id
	`, userID, newRefreshTokenHash, userAgent, ip, expiresAt).Scan(&id); err != nil {
		return "", err
	}
	return id, tx.Commit(ctx)
}

// --- Email verification / password reset tokens ---

func (r *Repository) CreateEmailVerificationToken(ctx context.Context, userID, tokenHash string, expiresAt time.Time) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO email_verification_tokens (user_id, token_hash, expires_at) VALUES ($1, $2, $3)
	`, userID, tokenHash, expiresAt)
	return err
}

func (r *Repository) ConsumeEmailVerificationToken(ctx context.Context, tokenHash string) (userID string, err error) {
	err = r.pool.QueryRow(ctx, `
		UPDATE email_verification_tokens SET used_at = now()
		WHERE token_hash = $1 AND used_at IS NULL AND expires_at > now()
		RETURNING user_id
	`, tokenHash).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	return userID, err
}

func (r *Repository) CreatePasswordResetToken(ctx context.Context, userID, tokenHash string, expiresAt time.Time) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO password_reset_tokens (user_id, token_hash, expires_at) VALUES ($1, $2, $3)
	`, userID, tokenHash, expiresAt)
	return err
}

func (r *Repository) ConsumePasswordResetToken(ctx context.Context, tokenHash string) (userID string, err error) {
	err = r.pool.QueryRow(ctx, `
		UPDATE password_reset_tokens SET used_at = now()
		WHERE token_hash = $1 AND used_at IS NULL AND expires_at > now()
		RETURNING user_id
	`, tokenHash).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	return userID, err
}
