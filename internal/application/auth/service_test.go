package auth

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	domainidentity "github.com/wecratfs/commerce/internal/domain/identity"
	"github.com/wecratfs/commerce/internal/ports"
)

type fakeUserRepository struct {
	credentials domainidentity.Credentials
	created     domainidentity.User
	createErr   error
	getErr      error
	failed      int
	reset       int
	seenContext context.Context
	seenEmail   string
}

func (f *fakeUserRepository) CreateCustomer(ctx context.Context, email, displayName, phone, passwordHash string) (domainidentity.User, error) {
	f.seenContext = ctx
	if f.createErr != nil {
		return domainidentity.User{}, f.createErr
	}
	f.created = domainidentity.User{Email: email, DisplayName: displayName}
	return f.created, nil
}

func (f *fakeUserRepository) GetCredentialsByEmail(ctx context.Context, email string) (domainidentity.Credentials, error) {
	f.seenContext = ctx
	f.seenEmail = email
	if f.getErr != nil {
		return domainidentity.Credentials{}, f.getErr
	}
	return f.credentials, nil
}

func TestLoginMapsSeededRoleUsernameToEmail(t *testing.T) {
	for _, username := range []string{"customer", "seller", "superadmin", "support", "operations", "finance"} {
		repository := &fakeUserRepository{getErr: ports.ErrCredentialsNotFound}
		_, _ = NewService(repository, fakeHasher{}).Login(context.Background(), strings.ToUpper(username), "wrong")
		if want := username + "@weevcrafts.test"; repository.seenEmail != want {
			t.Fatalf("%s looked up %s, want %s", username, repository.seenEmail, want)
		}
	}
}

func (f *fakeUserRepository) RecordFailedLogin(context.Context, int64) error {
	f.failed++
	return nil
}

func (f *fakeUserRepository) ResetFailedLogin(context.Context, int64) error {
	f.reset++
	return nil
}

type fakeHasher struct{}

func (fakeHasher) Hash(string) (string, error) { return "hash", nil }
func (fakeHasher) Compare(hash, password string) bool {
	return hash == "hash" && password == "correct-password"
}

func TestRegisterValidatesAndNormalizesInput(t *testing.T) {
	repository := &fakeUserRepository{}
	service := NewService(repository, fakeHasher{})
	ctx := context.Background()
	user, err := service.Register(ctx, RegisterInput{
		Email:         "  PERSON@Example.COM ",
		DisplayName:   " A maker ",
		Phone:         "+91 98765 43210",
		Password:      "correct-password",
		AcceptedTerms: true,
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if user.Email != "person@example.com" || user.DisplayName != "A maker" {
		t.Fatalf("unexpected normalized user: %+v", user)
	}
	if repository.seenContext != ctx {
		t.Fatal("register did not propagate context")
	}
}

func TestRegisterRequiresDisplayNameBetweenTwoAnd120Runes(t *testing.T) {
	for name, displayName := range map[string]string{"empty": " ", "one rune": "A", "too long": strings.Repeat("a", 121)} {
		t.Run(name, func(t *testing.T) {
			repository := &fakeUserRepository{}
			service := NewService(repository, fakeHasher{})
			_, err := service.Register(context.Background(), RegisterInput{Email: "person@example.test", DisplayName: displayName, Phone: "9876543210", Password: "correct-password", AcceptedTerms: true})
			if !errors.Is(err, ErrInvalidRegistration) {
				t.Fatalf("expected invalid display name rejection, got %v", err)
			}
			if repository.created.Email != "" {
				t.Fatal("invalid display name reached persistence")
			}
		})
	}
}

func TestRegisterRequiresTermsAcceptance(t *testing.T) {
	repository := &fakeUserRepository{}
	service := NewService(repository, fakeHasher{})
	if _, err := service.Register(context.Background(), RegisterInput{
		Email: "person@example.com", DisplayName: "A maker", Phone: "9876543210", Password: "correct-password",
	}); !errors.Is(err, ErrInvalidRegistration) {
		t.Fatalf("registration without consent should fail, got %v", err)
	}
	if repository.created.Email != "" {
		t.Fatal("registration without terms consent reached persistence")
	}
}

func TestLoginRejectsUnknownAndRecordsBadPassword(t *testing.T) {
	repository := &fakeUserRepository{getErr: ports.ErrCredentialsNotFound}
	service := NewService(repository, fakeHasher{})
	if _, err := service.Login(context.Background(), "missing@example.com", "anything"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected invalid credentials, got %v", err)
	}

	repository.getErr = nil
	repository.credentials = domainidentity.Credentials{
		User:         domainidentity.User{ID: 7, Status: "active"},
		PasswordHash: "hash",
	}
	if _, err := service.Login(context.Background(), "person@example.com", "wrong-password"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected invalid password, got %v", err)
	}
	if repository.failed != 1 {
		t.Fatalf("expected one failed-login record, got %d", repository.failed)
	}
}

func TestLoginResetsFailuresAndRejectsLockedAccount(t *testing.T) {
	repository := &fakeUserRepository{credentials: domainidentity.Credentials{
		User:           domainidentity.User{ID: 8, Status: "active"},
		PasswordHash:   "hash",
		LockedUntil:    nil,
		FailedAttempts: 3,
	}}
	service := NewService(repository, fakeHasher{})
	user, err := service.Login(context.Background(), "person@example.com", "correct-password")
	if err != nil || user.ID != 8 {
		t.Fatalf("expected successful login, user=%+v err=%v", user, err)
	}
	if repository.reset != 1 {
		t.Fatalf("expected one reset, got %d", repository.reset)
	}

	lockedUntil := time.Now().Add(time.Minute)
	repository.credentials.LockedUntil = &lockedUntil
	if _, err := service.Login(context.Background(), "person@example.com", "correct-password"); !errors.Is(err, ErrAccountLocked) {
		t.Fatalf("expected locked account, got %v", err)
	}
}
