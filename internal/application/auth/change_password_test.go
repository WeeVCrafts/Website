package auth

import (
	"context"
	"errors"
	identity "github.com/wecratfs/commerce/internal/domain/identity"
	"testing"
	"time"
)

type passwordChangeFake struct {
	fakeUserRepository
	changes  int
	expected string
}

func (f *passwordChangeFake) GetCredentialsByID(context.Context, int64) (identity.Credentials, error) {
	return f.credentials, nil
}
func (f *passwordChangeFake) ChangePassword(_ context.Context, _ int64, expected, _ string) error {
	f.changes++
	f.expected = expected
	return nil
}

func TestChangePasswordRequiresCurrentPasswordMatchingConfirmationAndUnlockedAccount(t *testing.T) {
	future := time.Now().Add(time.Hour)
	for _, tc := range []struct {
		name, current, next, confirmation string
		locked                            *time.Time
		want                              error
		changes, failed                   int
	}{
		{name: "success", current: "correct-password", next: "new-password-long", confirmation: "new-password-long", changes: 1},
		{name: "incorrect current password", current: "wrong", next: "new-password-long", confirmation: "new-password-long", want: ErrInvalidCredentials, failed: 1},
		{name: "confirmation mismatch", current: "correct-password", next: "new-password-long", confirmation: "different-password", want: ErrInvalidPasswordChange},
		{name: "short password", current: "correct-password", next: "short", confirmation: "short", want: ErrInvalidPasswordChange},
		{name: "unicode character minimum", current: "correct-password", next: "手工艺品密码", confirmation: "手工艺品密码", want: ErrInvalidPasswordChange},
		{name: "same password", current: "correct-password", next: "correct-password", confirmation: "correct-password", want: ErrInvalidPasswordChange},
		{name: "locked", current: "correct-password", next: "new-password-long", confirmation: "new-password-long", locked: &future, want: ErrAccountLocked},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := &passwordChangeFake{fakeUserRepository: fakeUserRepository{credentials: identity.Credentials{User: identity.User{ID: 7, Status: "active"}, PasswordHash: "hash", LockedUntil: tc.locked}}}
			err := NewService(f, fakeHasher{}).ChangePassword(context.Background(), 7, tc.current, tc.next, tc.confirmation)
			if !errors.Is(err, tc.want) || f.changes != tc.changes || f.failed != tc.failed {
				t.Fatalf("err=%v changes=%d failed=%d", err, f.changes, f.failed)
			}
			if f.changes > 0 && f.expected != "hash" {
				t.Fatal("expected current hash guard")
			}
		})
	}
}
