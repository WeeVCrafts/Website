package auth

import domainidentity "github.com/wecratfs/commerce/internal/domain/identity"

// Workspace is a user-facing destination available to an authenticated role.
type Workspace struct {
	Role  domainidentity.Role
	Label string
	Path  string
}

var portalWorkspaces = []Workspace{
	{Role: domainidentity.RoleCustomer, Label: "Customer account", Path: "/account"},
	{Role: domainidentity.RoleSellerOwner, Label: "Seller owner", Path: "/seller-admin"},
	{Role: domainidentity.RoleSellerStaff, Label: "Seller staff", Path: "/seller-admin"},
	{Role: domainidentity.RoleMarketplaceAdmin, Label: "Marketplace admin", Path: "/marketplace-admin"},
	{Role: domainidentity.RoleSuperAdmin, Label: "Super admin", Path: "/super-admin"},
	{Role: domainidentity.RoleFinanceOperator, Label: "Finance", Path: "/finance-portal"},
	{Role: domainidentity.RoleOperations, Label: "Returns operations", Path: "/operations-portal"},
	{Role: domainidentity.RoleSupportAgent, Label: "Support", Path: "/support-portal"},
}

// WorkspacesForRoles returns only recognized workspaces, in stable display order.
func WorkspacesForRoles(roles []domainidentity.Role) []Workspace {
	assigned := make(map[domainidentity.Role]struct{}, len(roles))
	for _, role := range roles {
		assigned[role] = struct{}{}
	}
	// Owner and staff access the same seller shell. If an account carries both
	// assignments, show its higher-privilege owner workspace only once.
	if _, owner := assigned[domainidentity.RoleSellerOwner]; owner {
		delete(assigned, domainidentity.RoleSellerStaff)
	}

	workspaces := make([]Workspace, 0, len(assigned))
	for _, workspace := range portalWorkspaces {
		if _, ok := assigned[workspace.Role]; ok {
			workspaces = append(workspaces, workspace)
		}
	}
	return workspaces
}

// LoginDestination directs one-role accounts to their portal and sends
// multi-role or unassigned accounts to the workspace chooser.
func LoginDestination(roles []domainidentity.Role) string {
	workspaces := WorkspacesForRoles(roles)
	if len(workspaces) != 1 {
		return "/workspaces"
	}
	return workspaces[0].Path
}
