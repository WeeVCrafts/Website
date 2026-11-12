package auth

import (
	"context"
	"errors"
	"github.com/wecratfs/commerce/internal/ports"
	"testing"
)

type firebaseMailFixture struct {
	proof           ports.FirebaseIdentity
	found, allowed  bool
	err             error
	calls           int
	existingAccount bool
	returnURL       string
}

func (f *firebaseMailFixture) FirebaseEmailRecipient(context.Context, int64, string) (ports.FirebaseIdentity, bool, error) {
	return f.proof, f.found, nil
}
func (f *firebaseMailFixture) FirebaseVerificationAllowed(context.Context, string) (bool, error) {
	return f.allowed, nil
}
func (f *firebaseMailFixture) PasswordResetLink(_ context.Context, _ ports.FirebaseIdentity, returnURL string) (string, error) {
	f.calls++
	f.returnURL = returnURL
	return "https://project.firebaseapp.com/__/auth/action?mode=resetPassword&oobCode=fixture&continueUrl=https%3A%2F%2Fweevcrafts.com%2Flogin", f.err
}

func (f *firebaseMailFixture) RecoveryEmailExists(context.Context, string) (bool, error) {
	return f.existingAccount, nil
}

func (f *firebaseMailFixture) UnfinishedPasswordIdentity(_ context.Context, email string) (ports.FirebaseIdentity, error) {
	if f.proof.Email != email {
		return ports.FirebaseIdentity{}, ports.ErrCredentialsNotFound
	}
	return f.proof, nil
}

func TestRecoverySendsFirebaseLinkForUnfinishedPasswordSignupOnlyWithoutLocalAccount(t *testing.T) {
	ctx := context.Background()
	email := "unfinished@example.invalid"
	f := &firebaseMailFixture{proof: ports.FirebaseIdentity{UID: "unfinished", Email: email}}
	mail := &fakeEmailSender{}
	s := NewRecoveryService(&fakeAuthTokenRepository{}, fakeHasher{}, mail, "https://weevcrafts.com").WithFirebaseEmails(f, f, "")
	if err := s.RequestPasswordReset(ctx, email); err != nil || mail.message.To != email || f.returnURL != "https://weevcrafts.com/login" {
		t.Fatalf("unfinished signup recovery failed: %v", err)
	}
	f.existingAccount = true
	mail.message = ports.EmailMessage{}
	if err := s.RequestPasswordReset(ctx, email); err != nil || mail.message.To != "" {
		t.Fatalf("unfinished Firebase recovery overlapped an existing local account: %v", err)
	}
}
func (f *firebaseMailFixture) EmailVerificationLink(context.Context, ports.FirebaseIdentity, string) (string, error) {
	f.calls++
	return "https://weevcrafts.com/verify-email?oobCode=fixture", f.err
}

func TestFirebaseRecoveryUsesBoundIdentityAndPreservesNativeFallback(t *testing.T) {
	native := &fakeAuthTokenRepository{}
	mail := &fakeEmailSender{}
	f := &firebaseMailFixture{proof: ports.FirebaseIdentity{UID: "fixture", Email: "bound@example.invalid"}, found: true, allowed: true}
	s := NewRecoveryService(native, fakeHasher{}, mail, "https://weevcrafts.com").WithFirebaseEmails(f, f, "https://weevcrafts.com/")
	if err := s.RequestPasswordReset(context.Background(), f.proof.Email); err != nil {
		t.Fatal(err)
	}
	if mail.message.To != f.proof.Email || mail.message.Subject != PasswordResetSubject || native.resetHash != "" {
		t.Fatal("Firebase reset did not use bound identity")
	}
	if mail.message.ActionURL != "https://weevcrafts.com/reset-password?mode=resetPassword&oobCode=fixture" {
		t.Fatalf("Firebase reset email did not use the storefront action: %s", mail.message.ActionURL)
	}
	f.err = ports.ErrCredentialsNotFound
	mail.message = ports.EmailMessage{}
	if err := s.RequestPasswordReset(context.Background(), f.proof.Email); err != nil || mail.message.To != "" {
		t.Fatal("missing identity leaked or delivered")
	}
	f.found = false
	native.found = true
	native.recipient = ports.EmailRecipient{Email: "native@example.invalid"}
	if err := s.RequestPasswordReset(context.Background(), native.recipient.Email); err != nil || mail.message.To != native.recipient.Email || native.resetHash == "" {
		t.Fatal("native fallback failed")
	}
}

func TestFirebaseVerificationRejectsDisallowedAccountsAndReportsDeliveryFailure(t *testing.T) {
	f := &firebaseMailFixture{proof: ports.FirebaseIdentity{UID: "fixture", Email: "bound@example.invalid"}, allowed: false}
	mail := &fakeEmailSender{}
	s := NewRecoveryService(&fakeAuthTokenRepository{}, fakeHasher{}, mail, "").WithFirebaseEmails(f, f, "https://weevcrafts.com/")
	if !errors.Is(s.RequestFirebaseVerification(context.Background(), f.proof), ErrRecoveryInvalidRequest) || f.calls != 0 {
		t.Fatal("disallowed account received email")
	}
	f.allowed = true
	f.proof.EmailVerified = true
	if err := s.RequestFirebaseVerification(context.Background(), f.proof); err != nil || f.calls != 0 {
		t.Fatal("verified identity received duplicate verification")
	}
	f.proof.EmailVerified = false
	mail.err = errors.New("provider failure")
	if !errors.Is(s.RequestFirebaseVerification(context.Background(), f.proof), ErrRecoveryUnavailable) {
		t.Fatal("delivery failure hidden")
	}
	if mail.message.To != f.proof.Email {
		t.Fatal("verification recipient changed")
	}
}
