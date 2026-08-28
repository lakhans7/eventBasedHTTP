package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/lakhans7/eventbasedhttp/internal/audit"
	"github.com/lakhans7/eventbasedhttp/internal/domain"
	"github.com/lakhans7/eventbasedhttp/internal/mailer"
)

var (
	ErrEmailAlreadyRegistered  = errors.New("an account with this email already exists")
	ErrInvalidCredentials      = errors.New("email or password is incorrect")
	ErrSessionRevokedOrExpired = errors.New("session is revoked or expired")
)

type Service struct {
	repo            *Repository
	jwt             *JWTIssuer
	mailer          mailer.Mailer
	audit           *audit.Service
	frontendOrigin  string
	refreshTokenTTL time.Duration
}

func NewService(repo *Repository, jwt *JWTIssuer, m mailer.Mailer, auditSvc *audit.Service, frontendOrigin string, refreshTokenTTL time.Duration) *Service {
	return &Service{repo: repo, jwt: jwt, mailer: m, audit: auditSvc, frontendOrigin: frontendOrigin, refreshTokenTTL: refreshTokenTTL}
}

type TokenPair struct {
	AccessToken  string
	RefreshToken string
	SessionID    string
}

func (s *Service) Register(ctx context.Context, email, name, password, ip, ua string) (*domain.User, error) {
	if _, err := s.repo.GetUserByEmail(ctx, email); err == nil {
		return nil, ErrEmailAlreadyRegistered
	} else if !errors.Is(err, ErrNotFound) {
		return nil, err
	}

	hash, err := HashPassword(password)
	if err != nil {
		return nil, err
	}

	user, err := s.repo.CreateUser(ctx, email, name, hash)
	if err != nil {
		return nil, err
	}

	s.audit.Log(ctx, audit.Entry{UserID: &user.ID, Action: "user.registered", ResourceType: "user", ResourceID: user.ID, IPAddress: ip, UserAgent: ua})

	if err := s.sendVerificationEmail(ctx, user); err != nil {
		// Registration still succeeds; the user can request a new verification email.
		return user, nil
	}
	return user, nil
}

func (s *Service) sendVerificationEmail(ctx context.Context, user *domain.User) error {
	token, err := GenerateOpaqueToken()
	if err != nil {
		return err
	}
	if err := s.repo.CreateEmailVerificationToken(ctx, user.ID, HashToken(token), time.Now().Add(24*time.Hour)); err != nil {
		return err
	}
	link := fmt.Sprintf("%s/verify-email.html?token=%s", s.frontendOrigin, token)
	return s.mailer.Send(user.Email, "Verify your email", "Click to verify your email: "+link)
}

func (s *Service) Authenticate(ctx context.Context, email, password string) (*domain.User, error) {
	user, err := s.repo.GetUserByEmail(ctx, email)
	if errors.Is(err, ErrNotFound) {
		return nil, ErrInvalidCredentials
	}
	if err != nil {
		return nil, err
	}
	if !VerifyPassword(user.PasswordHash, password) {
		return nil, ErrInvalidCredentials
	}
	if user.Status != domain.UserStatusActive {
		return nil, ErrInvalidCredentials
	}
	_ = s.repo.TouchLastLogin(ctx, user.ID)
	return user, nil
}

func (s *Service) IssueTokenPair(ctx context.Context, userID, ip, ua string) (*TokenPair, error) {
	refreshToken, err := GenerateOpaqueToken()
	if err != nil {
		return nil, err
	}
	sessionID, err := s.repo.CreateSession(ctx, userID, HashToken(refreshToken), ua, ip, time.Now().Add(s.refreshTokenTTL))
	if err != nil {
		return nil, err
	}
	accessToken, err := s.jwt.Issue(userID, sessionID)
	if err != nil {
		return nil, err
	}
	return &TokenPair{AccessToken: accessToken, RefreshToken: refreshToken, SessionID: sessionID}, nil
}

func (s *Service) Refresh(ctx context.Context, refreshToken, ip, ua string) (*TokenPair, error) {
	session, err := s.repo.GetSessionByRefreshHash(ctx, HashToken(refreshToken))
	if errors.Is(err, ErrNotFound) {
		return nil, ErrSessionRevokedOrExpired
	}
	if err != nil {
		return nil, err
	}
	if session.RevokedAt != nil || time.Now().After(session.ExpiresAt) {
		return nil, ErrSessionRevokedOrExpired
	}

	newRefreshToken, err := GenerateOpaqueToken()
	if err != nil {
		return nil, err
	}
	newSessionID, err := s.repo.ReplaceSession(ctx, session.ID, session.UserID, HashToken(newRefreshToken), ua, ip, time.Now().Add(s.refreshTokenTTL))
	if err != nil {
		return nil, err
	}
	accessToken, err := s.jwt.Issue(session.UserID, newSessionID)
	if err != nil {
		return nil, err
	}
	return &TokenPair{AccessToken: accessToken, RefreshToken: newRefreshToken, SessionID: newSessionID}, nil
}

func (s *Service) Logout(ctx context.Context, sessionID string) error {
	return s.repo.RevokeSession(ctx, sessionID)
}

func (s *Service) LogoutAllDevices(ctx context.Context, userID string) error {
	return s.repo.RevokeAllSessionsForUser(ctx, userID)
}

func (s *Service) VerifyEmail(ctx context.Context, token string) error {
	userID, err := s.repo.ConsumeEmailVerificationToken(ctx, HashToken(token))
	if errors.Is(err, ErrNotFound) {
		return errors.New("invalid or expired verification link")
	}
	if err != nil {
		return err
	}
	return s.repo.MarkEmailVerified(ctx, userID)
}

// ForgotPassword never reveals whether the email exists (see docs/api.md).
func (s *Service) ForgotPassword(ctx context.Context, email string) {
	user, err := s.repo.GetUserByEmail(ctx, email)
	if err != nil {
		return
	}
	token, err := GenerateOpaqueToken()
	if err != nil {
		return
	}
	if err := s.repo.CreatePasswordResetToken(ctx, user.ID, HashToken(token), time.Now().Add(1*time.Hour)); err != nil {
		return
	}
	link := fmt.Sprintf("%s/reset-password.html?token=%s", s.frontendOrigin, token)
	_ = s.mailer.Send(user.Email, "Reset your password", "Click to reset your password: "+link)
}

func (s *Service) ResetPassword(ctx context.Context, token, newPassword string) error {
	userID, err := s.repo.ConsumePasswordResetToken(ctx, HashToken(token))
	if errors.Is(err, ErrNotFound) {
		return errors.New("invalid or expired reset link")
	}
	if err != nil {
		return err
	}
	hash, err := HashPassword(newPassword)
	if err != nil {
		return err
	}
	if err := s.repo.UpdatePassword(ctx, userID, hash); err != nil {
		return err
	}
	// Resetting a password invalidates every existing session as a security measure.
	return s.repo.RevokeAllSessionsForUser(ctx, userID)
}

func (s *Service) DeleteAccount(ctx context.Context, userID string) error {
	if err := s.repo.SoftDeleteUser(ctx, userID); err != nil {
		return err
	}
	return s.repo.RevokeAllSessionsForUser(ctx, userID)
}

func (s *Service) GetUserByID(ctx context.Context, id string) (*domain.User, error) {
	return s.repo.GetUserByID(ctx, id)
}
