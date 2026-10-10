package postgres

import (
	"context"
	"fmt"
	"testing"
	"time"

	domainidentity "github.com/wecratfs/commerce/internal/domain/identity"
)

func TestUserChangesInvalidateSessionsAndEmailChangesClearVerification(t *testing.T) {
	pool := openIntegrationPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	var actorID, userID int64
	if err := pool.QueryRow(ctx, `INSERT INTO users(email,display_name,status,email_verified_at) VALUES($1,'Admin fixture','active',NOW()) RETURNING id`, "session-admin-"+suffix+"@example.invalid").Scan(&actorID); err != nil {
		t.Fatalf("create actor: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO user_roles(user_id,role_slug) VALUES($1,'super_admin')`, actorID); err != nil {
		t.Fatalf("grant super admin role: %v", err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO users(email,display_name,status,email_verified_at) VALUES($1,'Profile fixture','active',NOW()) RETURNING id`, "session-target-"+suffix+"@example.invalid").Scan(&userID); err != nil {
		t.Fatalf("create target: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO credentials(user_id,password_hash) VALUES($1,'old-hash')`, userID); err != nil {
		t.Fatalf("create target credentials: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO user_roles(user_id,role_slug) VALUES($1,'customer')`, userID); err != nil {
		t.Fatalf("grant customer role: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM audit_logs WHERE resource_id IN ($1,$2) OR actor_id IN ($1,$2)`, fmt.Sprint(actorID), fmt.Sprint(userID))
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id IN ($1,$2)`, actorID, userID)
	})

	oldSessionCreated := time.Now().Add(-time.Hour)
	repository := NewRoleAdministrationRepository(pool)
	// Correct an ambiguous protected identity without dropping its Super Admin role.
	if _, err := pool.Exec(ctx, `INSERT INTO user_roles(user_id,role_slug) VALUES($1,'customer')`, actorID); err != nil {
		t.Fatal(err)
	}
	if err := repository.UpdateRoleAssignment(ctx, actorID, actorID, 0, domainidentity.RoleSuperAdmin, true, "Correct protected role assignment"); err != nil {
		t.Fatal(err)
	}
	var protectedRoles int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM user_roles WHERE user_id=$1`, actorID).Scan(&protectedRoles); err != nil || protectedRoles != 1 {
		t.Fatal("protected role correction failed")
	}
	if changed, err := repository.UpdateRoleUser(ctx, actorID, userID, "session-target-"+suffix+"@example.invalid", "Renamed fixture", "active", "", "Integration display name edit"); err != nil || changed {
		t.Fatalf("display-name-only edit should not change security state: changed=%t err=%v", changed, err)
	}
	checker := NewUserRepository(pool)
	if active, err := checker.IsSessionActive(ctx, userID, oldSessionCreated); err != nil || !active {
		t.Fatalf("display-name-only edit should keep session active: active=%t err=%v", active, err)
	}
	if changed, err := repository.UpdateRoleUser(ctx, actorID, userID, "renamed-"+suffix+"@example.invalid", "Updated fixture", "active", "new-hash", "Integration account edit"); err != nil || !changed {
		t.Fatalf("update account: %v", err)
	}
	var verifiedAt, revokedAt, passwordChangedAt *time.Time
	if err := pool.QueryRow(ctx, `SELECT email_verified_at,sessions_revoked_at FROM users WHERE id=$1`, userID).Scan(&verifiedAt, &revokedAt); err != nil {
		t.Fatalf("read verification and revocation state: %v", err)
	}
	if verifiedAt != nil || revokedAt == nil {
		t.Fatalf("email verification/revocation state is wrong: verified=%v revoked=%v", verifiedAt, revokedAt)
	}
	if err := pool.QueryRow(ctx, `SELECT password_changed_at FROM credentials WHERE user_id=$1`, userID).Scan(&passwordChangedAt); err != nil {
		t.Fatalf("read password timestamp: %v", err)
	}
	if passwordChangedAt == nil {
		t.Fatal("password change timestamp was not set")
	}
	if active, err := checker.IsSessionActive(ctx, userID, oldSessionCreated); err != nil || active {
		t.Fatalf("old session should be invalid: active=%t err=%v", active, err)
	}
	var newSessionCreated time.Time
	if err := pool.QueryRow(ctx, `SELECT clock_timestamp() - INTERVAL '1 millisecond'`).Scan(&newSessionCreated); err != nil {
		t.Fatalf("get new session timestamp: %v", err)
	}
	if active, err := checker.IsSessionActive(ctx, userID, newSessionCreated); err != nil || !active {
		t.Fatalf("new session should be active: active=%t err=%v", active, err)
	}

	if err := repository.UpdateRoleAssignment(ctx, actorID, userID, 0, domainidentity.RoleFinanceOperator, true, "Integration role change"); err != nil {
		t.Fatalf("grant role: %v", err)
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM user_roles WHERE user_id=$1`, userID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("replacement retained combined privileges: %d %v", count, err)
	}
	if active, err := checker.IsSessionActive(ctx, userID, newSessionCreated); err != nil || active {
		t.Fatalf("role change should invalidate the old session: active=%t err=%v", active, err)
	}
	if changed, err := repository.UpdateRoleUser(ctx, actorID, userID, "renamed-"+suffix+"@example.invalid", "Updated fixture", "suspended", "", "Integration suspend"); err != nil || !changed {
		t.Fatalf("suspend account: %v", err)
	}
	if active, err := checker.IsSessionActive(ctx, userID, time.Now().Add(time.Minute)); err != nil || active {
		t.Fatalf("suspended account session should be rejected: active=%t err=%v", active, err)
	}
}

func TestIdentityAuditsAvoidCopyingPersonalEmail(t *testing.T) {
	pool := openIntegrationPool(t)
	ctx := context.Background()
	var actorID int64
	if err := pool.QueryRow(ctx, `INSERT INTO users(email,display_name,status) VALUES($1,'Audit fixture','active') RETURNING id`, fmt.Sprintf("audit-admin-%d@example.invalid", time.Now().UnixNano())).Scan(&actorID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO user_roles(user_id,role_slug) VALUES($1,'super_admin')`, actorID); err != nil {
		t.Fatal(err)
	}
	repository := NewRoleAdministrationRepository(pool)
	id, err := repository.CreateRoleUser(ctx, actorID, fmt.Sprintf("audit-target-%d@example.invalid", time.Now().UnixNano()), "Audit target", "fixture-hash", domainidentity.RoleCustomer, 0, "Verify minimal audit metadata")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM audit_logs WHERE actor_id=$1`, fmt.Sprint(actorID))
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id IN ($1,$2)`, actorID, id)
	})
	if err := repository.DeleteRoleUser(ctx, actorID, id, "Verify deletion audit metadata"); err != nil {
		t.Fatal(err)
	}
	var events, personalFields int
	if err := pool.QueryRow(ctx, `SELECT count(*),count(*) FILTER (WHERE metadata ? 'email' OR metadata ? 'phone' OR metadata ? 'password') FROM audit_logs WHERE actor_id=$1 AND resource_id=$2`, fmt.Sprint(actorID), fmt.Sprint(id)).Scan(&events, &personalFields); err != nil {
		t.Fatal(err)
	}
	if events != 2 || personalFields != 0 {
		t.Fatalf("unsafe audit events=%d personal_fields=%d", events, personalFields)
	}
}
