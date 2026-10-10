package auth

import (
	domainidentity "github.com/wecratfs/commerce/internal/domain/identity"
	"net/url"
	"strings"
)

// Workspace is a user-facing destination available to an authenticated role.
type Workspace struct {
	Role  domainidentity.Role
	Label string
	Path  string
}

var portalWorkspaces = []Workspace{
	{Role: domainidentity.RoleCustomer, Label: "Customer shop", Path: "/products"},
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

// LoginDestination fails closed for ambiguous or unassigned identities.
func LoginDestination(roles []domainidentity.Role) string {
	assigned := make(map[domainidentity.Role]bool)
	for _, role := range roles {
		assigned[role] = true
	}
	workspaces := WorkspacesForRoles(roles)
	if len(assigned) != 1 || len(workspaces) != 1 {
		return "/access-unavailable"
	}
	return workspaces[0].Path
}

// RedirectDestination is shared by HTML and JSON sign-in. Only customer
// destinations may honor a next URL, and only within the customer surface.
func RedirectDestination(roles []domainidentity.Role, next string) string {
	destination := LoginDestination(roles)
	if destination != "/products" {
		return destination
	}
	candidate := strings.TrimSpace(next)
	parsed, err := url.Parse(candidate)
	if err != nil || !strings.HasPrefix(candidate, "/") || strings.HasPrefix(candidate, "//") || strings.ContainsAny(candidate, "\\\r\n") || parsed.IsAbs() || parsed.Host != "" {
		return destination
	}
	path := parsed.Path
	if strings.ContainsAny(path, "\\\r\n") || strings.Contains(path, "/../") || strings.HasSuffix(path, "/..") {
		return destination
	}
	allowed := []string{"/products", "/category", "/categories", "/brands", "/makers", "/search", "/deals", "/wishlist", "/cart", "/checkout", "/account", "/orders", "/tracking", "/returns", "/payments"}
	if path == "/" {
		return candidate
	}
	for _, prefix := range allowed {
		if path == prefix || strings.HasPrefix(path, prefix+"/") {
			return candidate
		}
	}
	return destination
}
