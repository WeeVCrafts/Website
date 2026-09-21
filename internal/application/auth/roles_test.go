package auth

import (
	"context"
	"errors"
	"testing"

	domainidentity "github.com/wecratfs/commerce/internal/domain/identity"
	"github.com/wecratfs/commerce/internal/ports"
)

type roleAdminRepository struct{}

func (roleAdminRepository) ListRoleAssignments(context.Context, int64, string) ([]domainidentity.RoleAssignment, error) {
	return []domainidentity.RoleAssignment{{ID: 9, Roles: []string{"customer"}}}, nil
}
func (roleAdminRepository) UpdateRoleAssignment(context.Context, int64, int64, int64, domainidentity.Role, bool, string) error {
	return nil
}
func (roleAdminRepository) CreateRoleUser(context.Context, int64, string, string, string, domainidentity.Role, int64, string) (int64, error) {
	return 10, nil
}
func (roleAdminRepository) DeleteRoleUser(context.Context, int64, int64, string) error { return nil }

type roleAdminChecker struct{ super bool }

func (r roleAdminChecker) HasAnyRole(context.Context, int64, ...domainidentity.Role) (bool, error) {
	return r.super, nil
}

type profileAdminRepository struct {
	roleAdminRepository
	gotEmail, gotName, gotStatus, gotHash string
	gotReason                             string
	securityChanged                       bool
}

func (r *profileAdminRepository) UpdateRoleUser(_ context.Context, _, _ int64, email, name, status, passwordHash, reason string) (bool, error) {
	r.gotEmail, r.gotName, r.gotStatus, r.gotHash, r.gotReason = email, name, status, passwordHash, reason
	return r.securityChanged || passwordHash != "", nil
}

type sessionRevokerSpy struct{ calls int }

func (s *sessionRevokerSpy) RevokeAll(context.Context, int64) error { s.calls++; return nil }

type fixedHasher struct{}

func (fixedHasher) Hash(string) (string, error) { return "hashed", nil }
func (fixedHasher) Compare(string, string) bool { return false }

func TestRoleAdminServiceRequiresSuperAdmin(t *testing.T) {
	service := NewRoleAdminService(roleAdminRepository{}, roleAdminChecker{})
	if _, err := service.List(context.Background(), 42, ""); !errors.Is(err, ports.ErrForbidden) {
		t.Fatalf("expected role list denial, got %v", err)
	}
}

func TestRoleAdminServiceValidatesAssignableRoleAndReason(t *testing.T) {
	service := NewRoleAdminService(roleAdminRepository{}, roleAdminChecker{super: true})
	if err := service.Update(context.Background(), 42, 9, 0, "super_admin", true, "promote"); !errors.Is(err, ErrInvalidRoleAssignment) {
		t.Fatalf("expected super-admin assignment denial, got %v", err)
	}
	if err := service.Update(context.Background(), 42, 9, 0, "finance_operator", true, "no"); !errors.Is(err, ErrInvalidRoleAssignment) {
		t.Fatalf("expected short reason denial, got %v", err)
	}
	if err := service.Update(context.Background(), 42, 9, 0, "finance_operator", true, "finance responsibility"); err != nil {
		t.Fatalf("valid role grant failed: %v", err)
	}
	if err := service.Update(context.Background(), 42, 9, 0, "seller_staff", true, "staff responsibility"); !errors.Is(err, ErrInvalidRoleAssignment) {
		t.Fatalf("seller staff assignment without a seller should be rejected, got %v", err)
	}
	if err := service.Update(context.Background(), 42, 9, 12, "seller_staff", true, "staff responsibility"); err != nil {
		t.Fatalf("seller staff assignment with a seller should be accepted: %v", err)
	}
}

func TestRoleAdminServiceEditsAccountAndHashesOptionalPassword(t *testing.T) {
	repository := &profileAdminRepository{}
	revoker := &sessionRevokerSpy{}
	service := NewRoleAdminService(repository, roleAdminChecker{super: true}, fixedHasher{}).WithSessionRevoker(revoker)
	if err := service.UpdateUser(context.Background(), 42, 9, " User@Example.com ", "Updated User", "suspended", "a-new-password-longer-than-12", "user update"); err != nil {
		t.Fatalf("update user: %v", err)
	}
	if repository.gotEmail != "user@example.com" || repository.gotName != "Updated User" || repository.gotStatus != "suspended" || repository.gotHash != "hashed" || repository.gotReason != "user update" {
		t.Fatalf("incorrect profile update passed to repository: %+v", repository)
	}
	if revoker.calls != 1 {
		t.Fatalf("security-sensitive edit should revoke sessions once, got %d", revoker.calls)
	}
	if err := service.UpdateUser(context.Background(), 42, 9, "bad", "Updated User", "active", "", "valid reason"); !errors.Is(err, ErrInvalidRoleAssignment) {
		t.Fatalf("expected invalid email rejection, got %v", err)
	}
}

func TestRoleAdminServiceDoesNotRevokeSessionsForNameOnlyEdit(t *testing.T) {
	repository := &profileAdminRepository{}
	revoker := &sessionRevokerSpy{}
	service := NewRoleAdminService(repository, roleAdminChecker{super: true}, fixedHasher{}).WithSessionRevoker(revoker)
	if err := service.UpdateUser(context.Background(), 42, 9, "user@example.com", "New Display Name", "active", "", "name correction"); err != nil {
		t.Fatalf("name-only edit failed: %v", err)
	}
	if revoker.calls != 0 {
		t.Fatalf("name-only edit revoked sessions %d times", revoker.calls)
	}
}

func TestRoleAdminServiceDoesNotAllowSuspendingSelf(t *testing.T) {
	service := NewRoleAdminService(&profileAdminRepository{}, roleAdminChecker{super: true}, fixedHasher{})
	if err := service.UpdateUser(context.Background(), 42, 42, "admin@example.com", "Admin User", "suspended", "", "suspension reason"); !errors.Is(err, ports.ErrRoleState) {
		t.Fatalf("expected self-suspension rejection, got %v", err)
	}
}

type roleAdminTestHasher struct{}

func (roleAdminTestHasher) Hash(value string) (string, error) { return "hash:" + value, nil }
func (roleAdminTestHasher) Compare(hash, value string) bool   { return hash == "hash:"+value }

func TestRoleAdminServiceCreatesAssignableUsersAndProtectsSuperAdmin(t *testing.T) {
	service := NewRoleAdminService(roleAdminRepository{}, roleAdminChecker{super: true}, roleAdminTestHasher{})
	if err := service.CreateUser(context.Background(), 42, "new@example.test", "New Operator", "long-enough-password", "finance_operator", 0, "new finance login"); err != nil {
		t.Fatalf("valid role user create failed: %v", err)
	}
	if err := service.CreateUser(context.Background(), 42, "new@example.test", "New Operator", "short", "finance_operator", 0, "new finance login"); !errors.Is(err, ErrInvalidRoleAssignment) {
		t.Fatalf("short password should be rejected, got %v", err)
	}
	if err := service.CreateUser(context.Background(), 42, "new@example.test", "New Operator", "long-enough-password", "super_admin", 0, "promote"); !errors.Is(err, ErrInvalidRoleAssignment) {
		t.Fatalf("super-admin creation should stay bootstrap-controlled, got %v", err)
	}
	if err := service.CreateUser(context.Background(), 42, "new@example.test", "New Operator", "long-enough-password", "seller_staff", 0, "seller staff"); !errors.Is(err, ErrInvalidRoleAssignment) {
		t.Fatalf("seller staff without seller assignment should be rejected, got %v", err)
	}
}

func TestRoleAdminServiceProtectsSelfDeletionAndRequiresReason(t *testing.T) {
	service := NewRoleAdminService(roleAdminRepository{}, roleAdminChecker{super: true})
	if err := service.DeleteUser(context.Background(), 42, 42, "cleanup"); !errors.Is(err, ErrInvalidRoleAssignment) {
		t.Fatalf("self deletion should be rejected, got %v", err)
	}
	if err := service.DeleteUser(context.Background(), 42, 43, "no"); !errors.Is(err, ErrInvalidRoleAssignment) {
		t.Fatalf("short delete reason should be rejected, got %v", err)
	}
}
