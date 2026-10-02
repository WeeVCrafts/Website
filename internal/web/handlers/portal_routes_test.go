package handlers

import (
	"strings"
	"testing"

	domainidentity "github.com/wecratfs/commerce/internal/domain/identity"
)

func TestCanonicalPortalPathsRequireTheirOwnRole(t *testing.T) {
	tests := []struct {
		role domainidentity.Role
		path string
		want bool
	}{
		{domainidentity.RoleMarketplaceAdmin, "/marketplace-admin", true},
		{domainidentity.RoleMarketplaceAdmin, "/super-admin/security", false},
		{domainidentity.RoleMarketplaceAdmin, "/marketplace-admin/security", false},
		{domainidentity.RoleMarketplaceAdmin, "/marketplace-admin/roles", false},
		{domainidentity.RoleSuperAdmin, "/super-admin/security", true},
		{domainidentity.RoleSuperAdmin, "/super-admin/roles", true},
		{domainidentity.RoleFinanceOperator, "/finance-portal/refunds", true},
		{domainidentity.RoleFinanceOperator, "/finance-portal/security", false},
		{domainidentity.RoleFinanceOperator, "/operations-portal/returns", false},
		{domainidentity.RoleOperations, "/operations-portal/returns", true},
		{domainidentity.RoleOperations, "/operations-portal/finance", false},
		{domainidentity.RoleSupportAgent, "/support-portal/cases", true},
		{domainidentity.RoleSellerOwner, "/seller-admin/products", true},
		{domainidentity.RoleCustomer, "/seller-admin/products", false},
	}
	for _, test := range tests {
		t.Run(string(test.role)+test.path, func(t *testing.T) {
			if got := roleAllowsPortalPath(test.role, test.path); got != test.want {
				t.Fatalf("roleAllowsPortalPath(%q, %q) = %v, want %v", test.role, test.path, got, test.want)
			}
		})
	}
}

func TestRoleSpecificMutationRoutesDelegateToAuthorizedApplicationHandlers(t *testing.T) {
	tests := map[string]string{
		"/super-admin/roles/update":              "/admin/roles/update",
		"/super-admin/products/variants/create":  "/admin/products/variants/create",
		"/super-admin/products/variants/update":  "/admin/products/variants/update",
		"/super-admin/products/variants/archive": "/admin/products/variants/archive",
		"/marketplace-admin/sellers/status":      "/admin/sellers/status",
		"/marketplace-admin/products/review":     "/admin/products/review",
		"/operations-portal/returns/status":      "/admin/returns/status",
		"/finance-portal/returns/status":         "/admin/returns/status",
	}
	for path, want := range tests {
		if got := canonicalAdminMutationPath(path); got != want {
			t.Errorf("canonicalAdminMutationPath(%q) = %q, want %q", path, got, want)
		}
	}
}

func TestCustomerPagesRequireCustomerRole(t *testing.T) {
	tests := []struct {
		roles []domainidentity.Role
		want  bool
	}{
		{roles: []domainidentity.Role{domainidentity.RoleCustomer}, want: true},
		{roles: []domainidentity.Role{domainidentity.RoleSellerOwner}, want: false},
		{roles: []domainidentity.Role{domainidentity.RoleCustomer, domainidentity.RoleSellerStaff}, want: true},
	}
	for _, test := range tests {
		if got := customerRoleAllowed(test.roles); got != test.want {
			t.Errorf("customer role access for %v = %v, want %v", test.roles, got, test.want)
		}
	}
}

func TestSellerPortalRoutesUseAssignedPermissions(t *testing.T) {
	permissions := []string{"PRODUCT_READ", "INVENTORY_READ"}
	tests := []struct {
		path    string
		isOwner bool
		want    bool
	}{
		{path: "/seller-admin", want: true},
		{path: "/seller-admin/terms", want: true},
		{path: "/seller-admin/products", want: true},
		{path: "/seller-admin/inventory", want: true},
		{path: "/seller-admin/orders", want: false},
		{path: "/seller-admin/team", want: false},
		{path: "/seller-admin/team", isOwner: true, want: true},
		{path: "/seller-admin/messages", isOwner: true, want: false},
	}
	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			if got := sellerPortalAllowed(test.path, test.isOwner, permissions); got != test.want {
				t.Fatalf("sellerPortalAllowed(%q, owner=%v) = %v, want %v", test.path, test.isOwner, got, test.want)
			}
		})
	}
}

func TestCanonicalSellerMutationsMapOnlySupportedWorkflows(t *testing.T) {
	tests := map[string]string{
		"/seller-admin/products":                  "/seller/products",
		"/seller-admin/products/submit":           "/seller/products/submit",
		"/seller-admin/products/restore":          "/seller/products/restore",
		"/seller-admin/products/variants/create":  "/seller/products/variants/create",
		"/seller-admin/products/variants/update":  "/seller/products/variants/update",
		"/seller-admin/products/variants/archive": "/seller/products/variants/archive",
		"/seller-admin/products/42/edit":          "/seller/products/42/edit",
		"/seller-admin/inventory/adjust":          "/seller/inventory/adjust",
		"/seller-admin/orders/fulfill":            "/seller/orders/fulfill",
		"/seller-admin/team/add":                  "/seller/team/add",
		"/seller-admin/team/update":               "/seller/team/update",
		"/seller-admin/team/remove":               "/seller/team/remove",
		"/seller-admin/onboarding":                "/seller/onboard",
		"/seller-admin/settings":                  "/seller/settings/update",
		"/seller-admin/returns/status":            "",
		"/seller-admin/settings/notification":     "",
		"/seller-admin/orders/fulfill/arbitrary":  "",
		"/marketplace-admin/products":             "",
	}
	for path, want := range tests {
		if got := canonicalSellerMutationPath(path); got != want {
			t.Errorf("canonicalSellerMutationPath(%q) = %q, want %q", path, got, want)
		}
	}
}

func TestSellerMutationPermissionsAreEnforcedAtPortalBoundary(t *testing.T) {
	tests := []struct {
		path        string
		owner       bool
		permissions []string
		want        bool
	}{
		{path: "/seller-admin/products", permissions: []string{"PRODUCT_WRITE"}, want: true},
		{path: "/seller-admin/products", permissions: []string{"PRODUCT_READ"}, want: false},
		{path: "/seller-admin/products/variants/create", permissions: []string{"PRODUCT_WRITE"}, want: true},
		{path: "/seller-admin/products/variants/archive", permissions: []string{"PRODUCT_READ"}, want: false},
		{path: "/seller-admin/inventory/adjust", permissions: []string{"INVENTORY_WRITE"}, want: true},
		{path: "/seller-admin/orders/fulfill", permissions: []string{"ORDER_READ"}, want: false},
		{path: "/seller-admin/orders/fulfill", permissions: []string{"ORDER_FULFILL"}, want: true},
		{path: "/seller-admin/team/add", permissions: []string{"PRODUCT_WRITE"}, want: false},
		{path: "/seller-admin/team/add", owner: true, want: true},
	}
	for _, test := range tests {
		if got := sellerMutationAllowed(test.path, test.owner, test.permissions); got != test.want {
			t.Errorf("sellerMutationAllowed(%q, owner=%v, permissions=%v) = %v, want %v", test.path, test.owner, test.permissions, got, test.want)
		}
	}
}

func TestLegacyAdminURLsRedirectToTheAssignedRolePortal(t *testing.T) {
	tests := []struct {
		name  string
		path  string
		roles []domainidentity.Role
		want  string
	}{
		{name: "marketplace root", path: "/admin", roles: []domainidentity.Role{domainidentity.RoleMarketplaceAdmin}, want: "/marketplace-admin"},
		{name: "super admin security", path: "/admin/security", roles: []domainidentity.Role{domainidentity.RoleSuperAdmin}, want: "/super-admin/security"},
		{name: "finance report", path: "/admin/finance-reconciliation", roles: []domainidentity.Role{domainidentity.RoleFinanceOperator}, want: "/finance-portal/finance-reconciliation"},
		{name: "operations returns", path: "/admin/returns", roles: []domainidentity.Role{domainidentity.RoleOperations}, want: "/operations-portal/returns"},
		{name: "ambiguous root", path: "/admin", roles: []domainidentity.Role{domainidentity.RoleMarketplaceAdmin, domainidentity.RoleFinanceOperator}, want: "/workspaces"},
		{name: "unauthorized security", path: "/admin/security", roles: []domainidentity.Role{domainidentity.RoleMarketplaceAdmin}, want: ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := legacyAdminDestination(test.path, test.roles); got != test.want {
				t.Fatalf("legacyAdminDestination(%q, %v) = %q, want %q", test.path, test.roles, got, test.want)
			}
		})
	}
}

func TestLegacySellerAndSupportPortalsRedirectToCanonicalPages(t *testing.T) {
	for legacy, want := range map[string]string{
		"/seller":                  "/seller-admin",
		"/seller/products":         "/seller-admin/products",
		"/seller/products/new":     "/seller-admin/products/new",
		"/seller/products/42/edit": "/seller-admin/products/42/edit",
		"/seller/orders":           "/seller-admin/orders",
		"/support":                 "/support-portal",
		"/support/tickets":         "/support-portal/cases",
		"/support/customers":       "/support-portal/customers",
		"/support/orders":          "/support-portal/orders",
	} {
		var got string
		if strings.HasPrefix(legacy, "/seller") {
			got = legacySellerPortalDestination(legacy)
		} else {
			got = legacySupportPortalDestination(legacy)
		}
		if got != want {
			t.Errorf("legacy destination for %q = %q, want %q", legacy, got, want)
		}
	}
}
