package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/wecratfs/commerce/internal/ports"
)

var (
	ErrRecoveryInvalidRequest = errors.New("recovery request is invalid")
	ErrRecoveryUnavailable    = errors.New("recovery delivery is unavailable")
)

const (
	PasswordResetTTL         = 30 * time.Minute
	EmailVerificationTTL     = 24 * time.Hour
	PasswordResetSubject     = "Reset your Weevcrafts password"
	EmailVerificationSubject = "Verify your Weevcrafts email"
)

// RecoveryService owns one-time account actions. Raw tokens exist only long
// enough to build an email action URL; PostgreSQL receives only SHA-256 token
// hashes and the email sender is a separate external boundary.
type RecoveryService struct {
	repository         ports.AuthTokenRepository
	hasher             ports.PasswordHasher
	sender             ports.EmailSender
	baseURL            string
	now                func() time.Time
	newToken           func() (string, error)
	firebaseLinks      ports.FirebaseEmailLinks
	firebaseRepository ports.FirebaseRecoveryRepository
	firebaseReturnURL  string
}

func (s *RecoveryService) WithFirebaseEmails(links ports.FirebaseEmailLinks, repository ports.FirebaseRecoveryRepository, returnURL string) *RecoveryService {
	s.firebaseLinks = links
	s.firebaseRepository = repository
	s.firebaseReturnURL = returnURL
	return s
}

func NewRecoveryService(repository ports.AuthTokenRepository, hasher ports.PasswordHasher, sender ports.EmailSender, baseURL string) *RecoveryService {
	return &RecoveryService{repository: repository, hasher: hasher, sender: sender, baseURL: strings.TrimRight(baseURL, "/"), now: time.Now, newToken: randomToken}
}

// RequestPasswordReset is deliberately non-enumerating: an unknown active
// account produces the same successful result as a known account.
func (s *RecoveryService) RequestPasswordReset(ctx context.Context, email string) error {
	if !validEmail(normalizeEmail(email)) {
		return ErrRecoveryInvalidRequest
	}
	if s.repository == nil || s.sender == nil {
		return ErrRecoveryUnavailable
	}
	if s.firebaseLinks != nil && s.firebaseRepository != nil {
		proof, found, err := s.firebaseRepository.FirebaseEmailRecipient(ctx, 0, normalizeEmail(email))
		if err != nil {
			return ErrRecoveryUnavailable
		}
		if found {
			link, err := s.firebaseLinks.PasswordResetLink(ctx, proof, s.firebaseReturnURL)
			if errors.Is(err, ports.ErrCredentialsNotFound) {
				return nil
			}
			if err != nil {
				return ErrRecoveryUnavailable
			}
			if s.sender.Send(ctx, ports.EmailMessage{To: proof.Email, DisplayName: proof.Name, Subject: PasswordResetSubject, ActionURL: link}) != nil {
				return ErrRecoveryUnavailable
			}
			return nil
		}
	}
	rawToken, tokenHash, err := s.newActionToken()
	if err != nil {
		return ErrRecoveryUnavailable
	}
	recipient, found, err := s.repository.IssuePasswordReset(ctx, normalizeEmail(email), tokenHash, s.now().UTC().Add(PasswordResetTTL))
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	if err := s.sender.Send(ctx, ports.EmailMessage{To: recipient.Email, DisplayName: recipient.DisplayName, Subject: PasswordResetSubject, ActionURL: s.actionURL("/reset-password", rawToken)}); err != nil {
		_ = s.repository.RevokeAuthToken(ctx, tokenHash)
		return ErrRecoveryUnavailable
	}
	return nil
}

func (s *RecoveryService) ResetPassword(ctx context.Context, rawToken, password string) (int64, error) {
	if strings.TrimSpace(rawToken) == "" || len(password) < 12 || len(password) > 1024 || s.repository == nil || s.hasher == nil {
		return 0, ErrRecoveryInvalidRequest
	}
	passwordHash, err := s.hasher.Hash(password)
	if err != nil {
		return 0, err
	}
	return s.repository.ResetPassword(ctx, hashActionToken(rawToken), passwordHash)
}

func (s *RecoveryService) RequestEmailVerification(ctx context.Context, userID int64) error {
	if userID <= 0 {
		return ErrRecoveryInvalidRequest
	}
	if s.repository == nil || s.sender == nil {
		return ErrRecoveryUnavailable
	}
	if s.firebaseLinks != nil && s.firebaseRepository != nil {
		proof, found, err := s.firebaseRepository.FirebaseEmailRecipient(ctx, userID, "")
		if err != nil {
			return ErrRecoveryUnavailable
		}
		if found {
			return s.RequestFirebaseVerification(ctx, proof)
		}
	}
	rawToken, tokenHash, err := s.newActionToken()
	if err != nil {
		return ErrRecoveryUnavailable
	}
	recipient, err := s.repository.IssueEmailVerification(ctx, userID, tokenHash, s.now().UTC().Add(EmailVerificationTTL))
	if err != nil {
		return err
	}
	if err := s.sender.Send(ctx, ports.EmailMessage{To: recipient.Email, DisplayName: recipient.DisplayName, Subject: EmailVerificationSubject, ActionURL: s.actionURL("/verify-email", rawToken)}); err != nil {
		_ = s.repository.RevokeAuthToken(ctx, tokenHash)
		return ErrRecoveryUnavailable
	}
	return nil
}

// The transport supplies freshly verified Firebase proof, never a client email.
// This also supports verification before an existing customer can securely link.
func (s *RecoveryService) RequestFirebaseVerification(ctx context.Context, proof ports.FirebaseIdentity) error {
	if s.firebaseLinks == nil || s.firebaseRepository == nil || s.sender == nil {
		return ErrRecoveryUnavailable
	}
	if proof.UID == "" || !validEmail(proof.Email) {
		return ErrRecoveryInvalidRequest
	}
	allowed, err := s.firebaseRepository.FirebaseVerificationAllowed(ctx, proof.Email)
	if err != nil {
		return ErrRecoveryUnavailable
	}
	if !allowed {
		return ErrRecoveryInvalidRequest
	}
	if proof.EmailVerified {
		return nil
	}
	link, err := s.firebaseLinks.EmailVerificationLink(ctx, proof, s.firebaseReturnURL)
	if err != nil {
		return ErrRecoveryUnavailable
	}
	if s.sender.Send(ctx, ports.EmailMessage{To: proof.Email, DisplayName: proof.Name, Subject: EmailVerificationSubject, ActionURL: link}) != nil {
		return ErrRecoveryUnavailable
	}
	return nil
}

func (s *RecoveryService) VerifyEmail(ctx context.Context, rawToken string) (int64, error) {
	if strings.TrimSpace(rawToken) == "" || s.repository == nil {
		return 0, ErrRecoveryInvalidRequest
	}
	return s.repository.ConsumeEmailVerification(ctx, hashActionToken(rawToken))
}

func (s *RecoveryService) newActionToken() (string, string, error) {
	rawToken, err := s.newToken()
	if err != nil {
		return "", "", err
	}
	return rawToken, hashActionToken(rawToken), nil
}

func (s *RecoveryService) actionURL(path, rawToken string) string {
	return s.baseURL + path + "?token=" + url.QueryEscape(rawToken)
}

func hashActionToken(rawToken string) string {
	digest := sha256.Sum256([]byte(rawToken))
	return hex.EncodeToString(digest[:])
}
