package postgres

import (
	"context"
	"errors"
	"github.com/wecratfs/commerce/internal/ports"
	"testing"
	"time"
)

func TestPasswordChangeGuardsHashAndInvalidatesSessionsAndRecovery(t *testing.T) {
	pool := openIntegrationPool(t)
	f := newCommerceFixture(t, pool, 1, 2)
	ctx := context.Background()
	id := f.userIDs[0]
	if _, err := pool.Exec(ctx, `INSERT INTO credentials(user_id,password_hash) VALUES($1,'old-hash')`, id); err != nil {
		t.Fatal(err)
	}
	repo := NewUserRepository(pool)
	var email string
	if err := pool.QueryRow(ctx, `SELECT email FROM users WHERE id=$1`, id).Scan(&email); err != nil {
		t.Fatal(err)
	}
	tokenRepo := NewAuthActionRepository(pool)
	if _, _, err := tokenRepo.IssuePasswordReset(ctx, email, "password-change-test-token", time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	before := time.Now().Add(-time.Second)
	if err := repo.ChangePassword(ctx, id, "stale-hash", "new-hash"); !errors.Is(err, ports.ErrCredentialsNotFound) {
		t.Fatalf("stale hash error=%v", err)
	}
	if err := repo.ChangePassword(ctx, id, "old-hash", "new-hash"); err != nil {
		t.Fatal(err)
	}
	c, err := repo.GetCredentialsByID(ctx, id)
	if err != nil || c.PasswordHash != "new-hash" {
		t.Fatalf("credentials=%+v err=%v", c, err)
	}
	active, err := repo.IsSessionActive(ctx, id, before)
	if err != nil || active {
		t.Fatalf("previous session active=%v err=%v", active, err)
	}
	if _, err = tokenRepo.ResetPassword(ctx, "password-change-test-token", "reset-hash"); !errors.Is(err, ports.ErrAuthTokenInvalid) {
		t.Fatalf("old recovery token error=%v", err)
	}
}
