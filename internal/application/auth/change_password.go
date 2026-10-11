package auth

import (
	"context"
	"errors"
	"unicode/utf8"

	domainidentity "github.com/wecratfs/commerce/internal/domain/identity"
	"github.com/wecratfs/commerce/internal/ports"
)

var ErrInvalidPasswordChange = errors.New("choose a different password with at least 12 characters and matching confirmation")

// PasswordChangeRepository uses an expected hash to reject concurrent changes.
// Its commit also invalidates existing sessions and one-time recovery tokens.
type PasswordChangeRepository interface {
	GetCredentialsByID(context.Context, int64) (domainidentity.Credentials, error)
	ChangePassword(context.Context, int64, string, string) error
}

func (s *Service) ChangePassword(ctx context.Context, userID int64, current, next, confirmation string) error {
	repo, ok := s.repository.(PasswordChangeRepository)
	if !ok || userID <= 0 {
		return ports.ErrForbidden
	}
	if len(current) > 1024 || utf8.RuneCountInString(next) < 12 || len(next) > 1024 || next != confirmation || next == current {
		return ErrInvalidPasswordChange
	}
	credentials, err := repo.GetCredentialsByID(ctx, userID)
	if err != nil {
		return err
	}
	if credentials.User.ID != userID || credentials.User.Status != "active" {
		return ErrInvalidCredentials
	}
	if credentials.LockedUntil != nil && credentials.LockedUntil.After(s.now()) {
		return ErrAccountLocked
	}
	if !s.hasher.Compare(credentials.PasswordHash, current) {
		if err := s.repository.RecordFailedLogin(ctx, userID); err != nil {
			return err
		}
		return ErrInvalidCredentials
	}
	encoded, err := s.hasher.Hash(next)
	if err != nil {
		return err
	}
	return repo.ChangePassword(ctx, userID, credentials.PasswordHash, encoded)
}
