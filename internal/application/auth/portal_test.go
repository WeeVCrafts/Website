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
		{domainidentity.RoleCustomer, "/products"},
		{domainidentity.RoleSeller, "/seller-admin"},
		{domainidentity.RoleAdmin, "/admin"},
		{domainidentity.RoleSuperAdmin, "/super-admin"},
		{domainidentity.RoleAdmin, "/admin"},
		{domainidentity.RoleAdmin, "/admin"},
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
		domainidentity.RoleSeller,
		domainidentity.RoleAdmin,
		domainidentity.RoleSeller,
	}
	if got := LoginDestination(roles); got != "/access-unavailable" {
		t.Fatalf("LoginDestination(multiple roles) = %q, want /access-unavailable", got)
	}
}

func TestSellerOwnerAndStaffShareOneWorkspace(t *testing.T) {
	workspaces := WorkspacesForRoles([]domainidentity.Role{domainidentity.RoleSeller, domainidentity.Role("seller_staff")})
	if len(workspaces) != 1 || workspaces[0].Role != domainidentity.RoleSeller || workspaces[0].Path != "/seller-admin" {
		t.Fatalf("seller roles should share the owner workspace when both are assigned: %#v", workspaces)
	}
	if got := LoginDestination([]domainidentity.Role{domainidentity.RoleSeller, domainidentity.Role("seller_staff")}); got != "/access-unavailable" {
		t.Fatalf("LoginDestination() = %q, want /access-unavailable", got)
	}
}

func TestLoginDestinationDoesNotInventPortalForRolelessIdentity(t *testing.T) {
	if got := LoginDestination(nil); got != "/access-unavailable" {
		t.Fatalf("LoginDestination(no roles) = %q, want /access-unavailable", got)
	}
}
