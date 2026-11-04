package auth

import (
	"context"
	"errors"
	identity "github.com/wecratfs/commerce/internal/domain/identity"
	"github.com/wecratfs/commerce/internal/ports"
	"strings"
	"time"
)

var (
	ErrFirebaseInvalid      = errors.New("firebase authentication invalid")
	ErrFirebaseLinkRequired = errors.New("existing account requires password confirmation")
	ErrFirebaseTerms        = errors.New("accept the privacy notice and terms to create an account")
	ErrFirebaseRole         = errors.New("firebase customer access only")
)

type FirebaseService struct {
	provider   ports.FirebaseProvider
	repository ports.FirebaseIdentityRepository
	native     *Service
	checker    ports.UserSessionChecker
}

func NewFirebaseService(p ports.FirebaseProvider, r ports.FirebaseIdentityRepository, native *Service, checker ports.UserSessionChecker) *FirebaseService {
	return &FirebaseService{p, r, native, checker}
}

type FirebaseInput struct {
	Token         string `json:"id_token"`
	Phone         string `json:"phone"`
	AcceptedTerms bool   `json:"accepted_terms"`
	LinkPassword  string `json:"link_password"`
	Next          string `json:"next"`
}

func (s *FirebaseService) VerifyIdentity(ctx context.Context, raw string) (ports.FirebaseIdentity, error) {
	if len(raw) < 20 || len(raw) > 16384 {
		return ports.FirebaseIdentity{}, ErrFirebaseInvalid
	}
	proof, err := s.provider.Verify(ctx, raw)
	if err != nil || proof.UID == "" || !validEmail(proof.Email) || (proof.Provider != "google.com" && proof.Provider != "password") || proof.AuthTime.After(time.Now().Add(time.Minute)) || time.Since(proof.AuthTime) > 5*time.Minute {
		return ports.FirebaseIdentity{}, ErrFirebaseInvalid
	}
	proof.Email = strings.ToLower(strings.TrimSpace(proof.Email))
	return proof, nil
}

func (s *FirebaseService) Login(ctx context.Context, in FirebaseInput) (identity.User, error) {
	proof, err := s.VerifyIdentity(ctx, in.Token)
	if err != nil {
		return identity.User{}, err
	}
	var linkID int64
	if in.LinkPassword != "" {
		if !proof.EmailVerified {
			return identity.User{}, ErrFirebaseInvalid
		}
		user, err := s.native.Login(ctx, proof.Email, in.LinkPassword)
		if err != nil {
			return identity.User{}, ErrFirebaseInvalid
		}
		if len(user.Roles) != 1 || user.Roles[0] != identity.RoleCustomer {
			return identity.User{}, ErrFirebaseRole
		}
		linkID = user.ID
	}
	phone := strings.TrimSpace(in.Phone)
	if phone != "" && !validPhone(phone) {
		return identity.User{}, ErrInvalidRegistration
	}
	return s.repository.ResolveFirebase(ctx, proof, phone, in.AcceptedTerms, linkID)
}
func (s *FirebaseService) IsSessionActive(ctx context.Context, id int64, created time.Time) (bool, error) {
	active, err := s.checker.IsSessionActive(ctx, id, created)
	if err != nil || !active {
		return false, err
	}
	uid, email, err := s.repository.FirebaseBinding(ctx, id)
	if err != nil {
		return false, err
	}
	if uid == "" {
		return true, nil
	}
	return s.provider.ValidSession(ctx, uid, created, email)
}
