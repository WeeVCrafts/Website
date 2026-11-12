package postgres

import (
	"context"
	"errors"
	"testing"

	applicationauth "github.com/wecratfs/commerce/internal/application/auth"
	"github.com/wecratfs/commerce/internal/ports"
)

func TestNativeAndFirebaseRegistrationCannotClaimSameUsername(t *testing.T) {
	pool := isolatedRolePool(t, false)
	ctx := context.Background()
	users := NewUserRepository(pool)
	firebase := NewFirebaseRepository(pool)
	service := applicationauth.NewService(users, applicationauth.NewArgon2idHasher())
	start := make(chan struct{})
	results := make(chan error, 2)
	go func() {
		<-start
		_, err := service.Register(ctx, applicationauth.RegisterInput{
			Email: "native@example.invalid", DisplayName: "Shared name", Username: " Claim_Me ",
			Phone: "9876543210", Password: "test-password-123", AcceptedTerms: true,
		})
		results <- err
	}()
	go func() {
		<-start
		_, err := firebase.ResolveFirebase(ctx, ports.FirebaseIdentity{
			UID: "firebase-race-fixture", Email: "firebase@example.invalid", Name: "Shared name",
		}, "", "claim_me", true, 0)
		results <- err
	}()
	close(start)
	var succeeded, conflicts int
	for range 2 {
		switch err := <-results; {
		case err == nil:
			succeeded++
		case errors.Is(err, ports.ErrUsernameTaken):
			conflicts++
		default:
			t.Fatalf("unexpected signup outcome: %v", err)
		}
	}
	if succeeded != 1 || conflicts != 1 {
		t.Fatalf("successful signups=%d, username conflicts=%d", succeeded, conflicts)
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM users WHERE email IN ('native@example.invalid', 'firebase@example.invalid')`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("conflicting signup left a user record: count=%d err=%v", count, err)
	}
}

func TestUsernameAndEmailResolveSameIdentityAfterRoleChange(t *testing.T) {
	pool := isolatedRolePool(t, false)
	ctx := context.Background()
	id := roleFixture(t, pool, "customer")
	var email string
	if err := pool.QueryRow(ctx, `UPDATE users SET username='designated_customer' WHERE id=$1 RETURNING email`, id).Scan(&email); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO credentials(user_id,password_hash) VALUES($1,'opaque-hash')`, id); err != nil {
		t.Fatal(err)
	}
	repo := NewUserRepository(pool)
	for _, login := range []string{email, "designated_customer", " DESIGNATED_CUSTOMER "} {
		got, err := repo.GetCredentialsByEmail(ctx, login)
		if err != nil || got.User.ID != id || got.PasswordHash != "opaque-hash" {
			t.Fatalf("identity mismatch for %s: %v", login, err)
		}
	}
	if _, err := pool.Exec(ctx, `UPDATE user_roles SET role_slug='admin' WHERE user_id=$1`, id); err != nil {
		t.Fatal(err)
	}
	got, err := repo.GetCredentialsByEmail(ctx, "designated_customer")
	if err != nil || got.User.ID != id || len(got.User.Roles) != 1 || got.User.Roles[0] != "admin" {
		t.Fatalf("role change altered login identity: %v", err)
	}
	if _, err := repo.GetCredentialsByEmail(ctx, "missing_username"); !errors.Is(err, ports.ErrCredentialsNotFound) {
		t.Fatalf("unexpected unknown username result: %v", err)
	}
	other := roleFixture(t, pool, "customer")
	if _, err := pool.Exec(ctx, `UPDATE users SET username='designated_customer' WHERE id=$1`, other); err == nil {
		t.Fatal("duplicate username accepted")
	}
	if _, err := pool.Exec(ctx, `UPDATE users SET username='INVALID' WHERE id=$1`, other); err == nil {
		t.Fatal("unnormalized username accepted")
	}
}
