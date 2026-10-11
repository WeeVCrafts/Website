package handlers

import (
	identity "github.com/wecratfs/commerce/internal/domain/identity"
	"testing"
)

func TestFourRolePortalBoundaries(t *testing.T) {
	roles := []identity.Role{identity.RoleCustomer, identity.RoleSeller, identity.RoleAdmin, identity.RoleSuperAdmin}
	paths := []string{"/seller-admin/products", "/admin/orders", "/admin/support", "/admin/finance", "/super-admin/orders", "/super-admin/users", "/admin/users", "/admin/settings", "/admin/security"}
	for _, role := range roles {
		for _, path := range paths {
			want := false
			switch path {
			case "/seller-admin/products":
				want = role == identity.RoleSeller
			case "/admin/orders", "/admin/support", "/admin/finance":
				want = identity.CompanyRole(role)
			default:
				want = role == identity.RoleSuperAdmin
			}
			if got := roleAllowsPortalPath(role, path); got != want {
				t.Errorf("role %s path %s: %v want %v", role, path, got, want)
			}
		}
	}
}
func TestRetiredCompanyBookmarks(t *testing.T) {
	for _, path := range []string{"/marketplace-admin/orders", "/finance-portal/finance-reconciliation", "/operations-portal/returns"} {
		for _, role := range []identity.Role{identity.RoleAdmin, identity.RoleSuperAdmin} {
			got := legacyAdminDestination(path, []identity.Role{role})
			if got == "" {
				t.Errorf("no bookmark destination for %s %s", role, path)
			}
		}
	}
	if got := legacyAdminDestination("/marketplace-admin/security", []identity.Role{identity.RoleAdmin}); got != "" {
		t.Fatalf("restricted bookmark authorized: %s", got)
	}
}
func TestRetiredTeamRoutesCannotAuthorize(t *testing.T) {
	for _, owner := range []bool{false, true} {
		if sellerPortalAllowed("/seller-admin/team", owner, []string{"PRODUCT_WRITE"}) || sellerMutationAllowed("/seller-admin/team/add", owner, nil) {
			t.Fatal("retired team exposed")
		}
	}
	for _, path := range []string{"/seller-admin/team/add", "/seller-admin/team/update", "/seller-admin/team/remove"} {
		if canonicalSellerMutationPath(path) != "" {
			t.Fatal("retired team alias exposed")
		}
	}
	if sellerPortalAllowed("/seller-admin/products", false, []string{"PRODUCT_READ"}) {
		t.Fatal("membership grants seller access")
	}
	if !sellerPortalAllowed("/seller-admin/products", true, nil) {
		t.Fatal("owner denied")
	}
}
func TestCanonicalMutationAliases(t *testing.T) {
	for _, root := range []string{"/admin", "/super-admin"} {
		for _, action := range []string{"sellers/status", "products/review", "products/variants/create", "products/variants/update", "products/variants/archive", "returns/status"} {
			want := "/admin/" + action
			if got := canonicalAdminMutationPath(root + "/" + action); got != want {
				t.Errorf("%s: %s want %s", action, got, want)
			}
		}
	}
	if canonicalSellerMutationPath("/seller-admin/products/variants/create") != "/seller/products/variants/create" {
		t.Fatal("variant alias missing")
	}
}
func TestCustomerPagesRequireExactlyCustomerRole(t *testing.T) {
	if !customerRoleAllowed([]identity.Role{identity.RoleCustomer}) || customerRoleAllowed([]identity.Role{identity.RoleCustomer, identity.RoleSeller}) || customerRoleAllowed(nil) {
		t.Fatal("customer boundary")
	}
}
