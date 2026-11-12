package auth

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"

	domainidentity "github.com/wecratfs/commerce/internal/domain/identity"
	"github.com/wecratfs/commerce/internal/ports"
)

var (
	ErrInvalidUsername     = errors.New("username must start with a letter and contain 3 to 32 letters, digits, or underscores")
	ErrInvalidRegistration = errors.New("registration details are invalid")
	ErrInvalidCredentials  = errors.New("email or password is incorrect")
	ErrAccountLocked       = errors.New("account is temporarily locked")
)

type Service struct {
	repository            ports.UserRepository
	hasher                ports.PasswordHasher
	now                   func() time.Time
	firebaseUsernameLogin *FirebaseService
}

func NewService(repository ports.UserRepository, hasher ports.PasswordHasher) *Service {
	return &Service{repository: repository, hasher: hasher, now: time.Now}
}

const UsernameTakenMessage = "This username is already registered. Choose another."

var usernamePattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]{2,31}$`)

func normalizeUsername(value string) (string, error) {
	value = strings.TrimSpace(value)
	if !usernamePattern.MatchString(value) {
		return "", ErrInvalidUsername
	}
	return strings.ToLower(value), nil
}

type RegisterInput struct {
	Username      string
	Email         string
	DisplayName   string
	Phone         string
	Password      string
	AcceptedTerms bool
}

func (s *Service) Register(ctx context.Context, input RegisterInput) (domainidentity.User, error) {
	email := normalizeEmail(input.Email)
	displayName := strings.TrimSpace(input.DisplayName)
	phone := strings.TrimSpace(input.Phone)
	if !validEmail(email) || len([]rune(displayName)) < 2 || len([]rune(displayName)) > 120 || !validPhone(phone) || len(input.Password) < 12 || len(input.Password) > 1024 || !input.AcceptedTerms {
		return domainidentity.User{}, ErrInvalidRegistration
	}
	username, err := normalizeUsername(input.Username)
	if err != nil {
		return domainidentity.User{}, err
	}
	passwordHash, err := s.hasher.Hash(input.Password)
	if err != nil {
		return domainidentity.User{}, err
	}
	return s.repository.CreateCustomer(ctx, email, displayName, phone, passwordHash, username)
}

func validPhone(phone string) bool {
	digits := 0
	for index, char := range phone {
		if char >= '0' && char <= '9' {
			digits++
			continue
		}
		if char == '+' && index == 0 {
			continue
		}
		if char != ' ' && char != '(' && char != ')' && char != '-' {
			return false
		}
	}
	return len(phone) <= 24 && digits >= 7 && digits <= 15
}

func (s *Service) WithFirebaseUsernameLogin(firebase *FirebaseService) {
	s.firebaseUsernameLogin = firebase
}

func (s *Service) LoginWithUsername(ctx context.Context, login, password string) (domainidentity.User, error) {
	user, err := s.Login(ctx, login, password)
	if s.firebaseUsernameLogin != nil && errors.Is(err, ErrInvalidCredentials) {
		return s.firebaseUsernameLogin.loginWithPassword(ctx, login, password)
	}
	return user, err
}

func (s *Service) Login(ctx context.Context, email, password string) (domainidentity.User, error) {
	login := normalizeEmail(email)
	credentials, err := s.repository.GetCredentialsByEmail(ctx, login)
	if errors.Is(err, ports.ErrCredentialsNotFound) {
		return domainidentity.User{}, ErrInvalidCredentials
	}
	if err != nil {
		return domainidentity.User{}, err
	}
	if credentials.User.Status != "active" {
		return domainidentity.User{}, ErrInvalidCredentials
	}
	if credentials.LockedUntil != nil && credentials.LockedUntil.After(s.now()) {
		return domainidentity.User{}, ErrAccountLocked
	}
	if !s.hasher.Compare(credentials.PasswordHash, password) {
		if err := s.repository.RecordFailedLogin(ctx, credentials.User.ID); err != nil {
			return domainidentity.User{}, err
		}
		return domainidentity.User{}, ErrInvalidCredentials
	}
	if err := s.repository.ResetFailedLogin(ctx, credentials.User.ID); err != nil {
		return domainidentity.User{}, err
	}
	return credentials.User, nil
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func validEmail(email string) bool {
	if len(email) < 3 || len(email) > 320 {
		return false
	}
	parts := strings.Split(email, "@")
	return len(parts) == 2 && parts[0] != "" && parts[1] != "" && !strings.ContainsAny(email, " \t\r\n")
}
