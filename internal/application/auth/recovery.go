package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log/slog"
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

const PasswordResetAcceptedMessage = "If this email has a password, we’ll send a reset link. If you only sign in with Google, recover your password through Google."

type firebaseUnfinishedRecovery interface {
	UnfinishedPasswordIdentity(context.Context, string) (ports.FirebaseIdentity, error)
}

type recoveryAccountLookup interface {
	RecoveryEmailExists(context.Context, string) (bool, error)
}

func recoveryUnavailable(ctx context.Context, stage string) error {
	slog.WarnContext(ctx, "password recovery failed", "stage", stage, "request_id", ports.RequestID(ctx))
	return ErrRecoveryUnavailable
}

func (s *RecoveryService) sendFirebaseReset(ctx context.Context, proof ports.FirebaseIdentity) error {
	link, err := s.firebaseLinks.PasswordResetLink(ctx, proof, s.baseURL+"/login")
	if errors.Is(err, ports.ErrCredentialsNotFound) {
		return nil
	}
	if err != nil {
		return recoveryUnavailable(ctx, "firebase_link")
	}
	if s.sender.Send(ctx, ports.EmailMessage{To: proof.Email, DisplayName: proof.Name, Subject: PasswordResetSubject, ActionURL: s.firebaseResetURL(link)}) != nil {
		return recoveryUnavailable(ctx, "firebase_send")
	}
	return nil
}

func (s *RecoveryService) firebaseResetURL(link string) string {
	base, err := url.Parse(s.baseURL)
	if err != nil || base.Scheme != "https" || base.Host == "" {
		return link
	}
	action, err := url.Parse(link)
	if err != nil || action.Scheme != "https" || action.Host == "" {
		return link
	}
	params := action.Query()
	if params.Get("mode") != "resetPassword" || params.Get("oobCode") == "" {
		return link
	}
	return s.baseURL + "/reset-password?" + url.Values{
		"mode": {"resetPassword"}, "oobCode": {params.Get("oobCode")},
	}.Encode()
}

func (s *RecoveryService) recoverUnfinishedSignup(ctx context.Context, email string) error {
	provider, canFind := s.firebaseLinks.(firebaseUnfinishedRecovery)
	repo, canCheck := s.firebaseRepository.(recoveryAccountLookup)
	if !canFind || !canCheck {
		return nil
	}
	exists, err := repo.RecoveryEmailExists(ctx, email)
	if err != nil {
		return recoveryUnavailable(ctx, "unfinished_lookup")
	}
	if exists {
		return nil
	}
	proof, err := provider.UnfinishedPasswordIdentity(ctx, email)
	if errors.Is(err, ports.ErrCredentialsNotFound) {
		return nil
	}
	if err != nil {
		return recoveryUnavailable(ctx, "unfinished_firebase_lookup")
	}
	return s.sendFirebaseReset(ctx, proof)
}

// RequestPasswordReset is deliberately non-enumerating: an unknown active
// account produces the same successful result as a known account.
func (s *RecoveryService) RequestPasswordReset(ctx context.Context, email string) error {
	email = normalizeEmail(email)
	if !validEmail(email) {
		return ErrRecoveryInvalidRequest
	}
	if s.repository == nil || s.sender == nil {
		return recoveryUnavailable(ctx, "sender_configuration")
	}
	if s.firebaseLinks != nil && s.firebaseRepository != nil {
		proof, found, err := s.firebaseRepository.FirebaseEmailRecipient(ctx, 0, email)
		if err != nil {
			return recoveryUnavailable(ctx, "firebase_lookup")
		}
		if found {
			return s.sendFirebaseReset(ctx, proof)
		}
	}
	rawToken, tokenHash, err := s.newActionToken()
	if err != nil {
		return recoveryUnavailable(ctx, "native_token")
	}
	recipient, found, err := s.repository.IssuePasswordReset(ctx, email, tokenHash, s.now().UTC().Add(PasswordResetTTL))
	if err != nil {
		return recoveryUnavailable(ctx, "native_lookup_or_issue")
	}
	if !found {
		return s.recoverUnfinishedSignup(ctx, email)
	}
	if err := s.sender.Send(ctx, ports.EmailMessage{To: recipient.Email, DisplayName: recipient.DisplayName, Subject: PasswordResetSubject, ActionURL: s.actionURL("/reset-password", rawToken)}); err != nil {
		_ = s.repository.RevokeAuthToken(ctx, tokenHash)
		return recoveryUnavailable(ctx, "native_send")
	}
	return nil
}

type passwordResetTokenChecker interface {
	PasswordResetTokenActive(context.Context, string) (bool, error)
}

func (s *RecoveryService) PasswordResetLinkActive(ctx context.Context, token string) (bool, error) {
	if token == "" || len(token) > 256 {
		return false, nil
	}
	checker, ok := s.repository.(passwordResetTokenChecker)
	if !ok {
		return true, nil
	}
	return checker.PasswordResetTokenActive(ctx, hashActionToken(token))
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
