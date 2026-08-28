package auth

import (
	"context"
	"errors"

	"github.com/lakhans7/eventbasedhttp/internal/domain"
)

// LoginOrRegisterWithGoogle links an already-verified Google identity to an
// existing user (matched by Google subject, then by email) or creates a new
// user. Because Google verifies the email itself, we trust EmailVerified
// and mark the local account verified immediately.
func (s *Service) LoginOrRegisterWithGoogle(ctx context.Context, info *GoogleUserInfo) (*domain.User, error) {
	if user, err := s.repo.GetUserByGoogleID(ctx, info.Sub); err == nil {
		return user, nil
	} else if !errors.Is(err, ErrNotFound) {
		return nil, err
	}

	if user, err := s.repo.GetUserByEmail(ctx, info.Email); err == nil {
		if linkErr := s.repo.LinkGoogleIdentity(ctx, user.ID, info.Sub); linkErr != nil {
			return nil, linkErr
		}
		return user, nil
	} else if !errors.Is(err, ErrNotFound) {
		return nil, err
	}

	user, err := s.repo.CreateUserFromGoogle(ctx, info.Email, info.Name, info.Picture)
	if err != nil {
		return nil, err
	}
	if err := s.repo.LinkGoogleIdentity(ctx, user.ID, info.Sub); err != nil {
		return nil, err
	}
	return user, nil
}
