package auth

import (
	"context"
	"errors"
	"testing"

	domainidentity "github.com/wecratfs/commerce/internal/domain/identity"
	"github.com/wecratfs/commerce/internal/ports"
)

type roleAdminRepository struct{}

func (roleAdminRepository) ListRoleAssignments(context.Context, int64) ([]domainidentity.RoleAssignment, error) {
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

func TestRoleAdminServiceRequiresSuperAdmin(t *testing.T) {
	service := NewRoleAdminService(roleAdminRepository{}, roleAdminChecker{})
	if _, err := service.List(context.Background(), 42); !errors.Is(err, ports.ErrForbidden) {
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
