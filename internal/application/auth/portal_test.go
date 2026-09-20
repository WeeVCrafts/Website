package auth

import (
	"testing"

	domainidentity "github.com/wecratfs/commerce/internal/domain/identity"
)

func TestLoginDestinationRoutesSingleRoleToItsPortal(t *testing.T) {
	tests := []struct {
		role domainidentity.Role
		want string
	}{
		{domainidentity.RoleCustomer, "/account"},
		{domainidentity.RoleSellerOwner, "/seller-admin"},
		{domainidentity.RoleSellerStaff, "/seller-admin"},
		{domainidentity.RoleMarketplaceAdmin, "/marketplace-admin"},
		{domainidentity.RoleSuperAdmin, "/super-admin"},
		{domainidentity.RoleFinanceOperator, "/finance-portal"},
		{domainidentity.RoleOperations, "/operations-portal"},
		{domainidentity.RoleSupportAgent, "/support-portal"},
	}
	for _, test := range tests {
		t.Run(string(test.role), func(t *testing.T) {
			if got := LoginDestination([]domainidentity.Role{test.role}); got != test.want {
				t.Fatalf("LoginDestination(%q) = %q, want %q", test.role, got, test.want)
			}
		})
	}
}

func TestLoginDestinationRequiresWorkspaceChoiceForMultipleRoles(t *testing.T) {
	roles := []domainidentity.Role{
		domainidentity.RoleSellerOwner,
		domainidentity.RoleFinanceOperator,
		domainidentity.RoleSellerOwner,
	}
	if got := LoginDestination(roles); got != "/workspaces" {
		t.Fatalf("LoginDestination(multiple roles) = %q, want /workspaces", got)
	}
}

func TestSellerOwnerAndStaffShareOneWorkspace(t *testing.T) {
	workspaces := WorkspacesForRoles([]domainidentity.Role{domainidentity.RoleSellerOwner, domainidentity.RoleSellerStaff})
	if len(workspaces) != 1 || workspaces[0].Role != domainidentity.RoleSellerOwner || workspaces[0].Path != "/seller-admin" {
		t.Fatalf("seller roles should share the owner workspace when both are assigned: %#v", workspaces)
	}
	if got := LoginDestination([]domainidentity.Role{domainidentity.RoleSellerOwner, domainidentity.RoleSellerStaff}); got != "/seller-admin" {
		t.Fatalf("LoginDestination() = %q, want /seller-admin", got)
	}
}

func TestLoginDestinationDoesNotInventPortalForRolelessIdentity(t *testing.T) {
	if got := LoginDestination(nil); got != "/workspaces" {
		t.Fatalf("LoginDestination(no roles) = %q, want /workspaces", got)
	}
}
