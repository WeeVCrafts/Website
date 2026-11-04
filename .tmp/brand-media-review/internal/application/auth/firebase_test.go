package auth

import (
	"context"
	"errors"
	identity "github.com/wecratfs/commerce/internal/domain/identity"
	"github.com/wecratfs/commerce/internal/ports"
	"strings"
	"testing"
	"time"
)

type firebaseProof struct {
	proof ports.FirebaseIdentity
	err   error
	valid bool
}

func (f firebaseProof) Verify(context.Context, string) (ports.FirebaseIdentity, error) {
	return f.proof, f.err
}
func (f firebaseProof) ValidSession(context.Context, string, time.Time, string) (bool, error) {
	return f.valid, f.err
}
func (f firebaseProof) DeleteIdentity(context.Context, string) error { return nil }
func (f firebaseProof) SendPush(context.Context, string) error       { return nil }

type firebaseIdentityStore struct {
	called bool
	linkID int64
	uid    string
	err    error
}

func (r *firebaseIdentityStore) ResolveFirebase(_ context.Context, _ ports.FirebaseIdentity, _ string, _ bool, id int64) (identity.User, error) {
	r.called = true
	r.linkID = id
	return identity.User{ID: 12, Roles: []identity.Role{identity.RoleCustomer}}, r.err
}
func (r *firebaseIdentityStore) FirebaseBinding(context.Context, int64) (string, string, error) {
	return r.uid, "customer@example.invalid", r.err
}

type firebaseSessionCheck bool

func (s firebaseSessionCheck) IsSessionActive(context.Context, int64, time.Time) (bool, error) {
	return bool(s), nil
}
func TestFirebaseRejectsInvalidProofBeforeRepository(t *testing.T) {
	good := ports.FirebaseIdentity{UID: "firebase-fixture", Email: "customer@example.invalid", Provider: "google.com", EmailVerified: true, AuthTime: time.Now()}
	for _, name := range []string{"invalid signature", "stale", "future", "unsupported provider", "missing email", "missing uid"} {
		t.Run(name, func(t *testing.T) {
			proof := good
			var err error
			switch name {
			case "invalid signature":
				err = errors.New("invalid")
			case "stale":
				proof.AuthTime = time.Now().Add(-6 * time.Minute)
			case "future":
				proof.AuthTime = time.Now().Add(2 * time.Minute)
			case "unsupported provider":
				proof.Provider = "anonymous"
			case "missing email":
				proof.Email = ""
			case "missing uid":
				proof.UID = ""
			}
			store := &firebaseIdentityStore{}
			s := NewFirebaseService(firebaseProof{proof: proof, err: err}, store, nil, firebaseSessionCheck(true))
			_, err = s.Login(context.Background(), FirebaseInput{Token: strings.Repeat("x", 40), AcceptedTerms: true})
			if !errors.Is(err, ErrFirebaseInvalid) || store.called {
				t.Fatal("untrusted identity reached account repository")
			}
		})
	}
}
func TestFirebaseSessionRevocationAndNativeCompatibility(t *testing.T) {
	for _, tc := range []struct {
		name, uid        string
		db, remote, want bool
	}{{"native", "", true, false, true}, {"revoked locally", "uid", false, true, false}, {"revoked remotely", "uid", true, false, false}, {"active firebase", "uid", true, true, true}} {
		t.Run(tc.name, func(t *testing.T) {
			s := NewFirebaseService(firebaseProof{valid: tc.remote}, &firebaseIdentityStore{uid: tc.uid}, nil, firebaseSessionCheck(tc.db))
			active, err := s.IsSessionActive(context.Background(), 12, time.Now())
			if err != nil || active != tc.want {
				t.Fatalf("session active=%v error=%v", active, err)
			}
		})
	}
}
func TestFirebaseLinkRequiresVerifiedEmailAndCustomerRole(t *testing.T) {
	for _, verified := range []bool{false, true} {
		native := NewService(&fakeUserRepository{credentials: identity.Credentials{User: identity.User{ID: 99, Status: "active", Roles: []identity.Role{identity.RoleSuperAdmin}}, PasswordHash: "hash"}}, fakeHasher{})
		store := &firebaseIdentityStore{}
		s := NewFirebaseService(firebaseProof{proof: ports.FirebaseIdentity{UID: "fixture", Email: "admin@example.invalid", Provider: "google.com", EmailVerified: verified, AuthTime: time.Now()}}, store, native, firebaseSessionCheck(true))
		if _, err := s.Login(context.Background(), FirebaseInput{Token: strings.Repeat("x", 40), LinkPassword: "correct-password"}); err == nil || store.called {
			t.Fatal("operational or unverified identity linked")
		}
	}
}

func TestFirebaseCustomerLinkUsesExistingPassword(t *testing.T) {
	native := NewService(&fakeUserRepository{credentials: identity.Credentials{User: identity.User{ID: 99, Status: "active", Roles: []identity.Role{identity.RoleCustomer}}, PasswordHash: "hash"}}, fakeHasher{})
	store := &firebaseIdentityStore{}
	service := NewFirebaseService(firebaseProof{proof: ports.FirebaseIdentity{UID: "fixture", Email: "customer@example.invalid", Provider: "google.com", EmailVerified: true, AuthTime: time.Now()}}, store, native, firebaseSessionCheck(true))
	if _, err := service.Login(context.Background(), FirebaseInput{Token: strings.Repeat("x", 40), LinkPassword: "wrong-password"}); err == nil || store.called {
		t.Fatal("incorrect existing password linked account")
	}
	if _, err := service.Login(context.Background(), FirebaseInput{Token: strings.Repeat("x", 40), LinkPassword: "correct-password"}); err != nil || store.linkID != 99 {
		t.Fatal("verified customer password did not preserve the existing account")
	}
}
