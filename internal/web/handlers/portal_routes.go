package handlers

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"

	applicationauth "github.com/wecratfs/commerce/internal/application/auth"
	domaincommerce "github.com/wecratfs/commerce/internal/domain/commerce"
	domainidentity "github.com/wecratfs/commerce/internal/domain/identity"
)

func roleAllowsPortalPath(role domainidentity.Role, path string) bool {
	switch {
	case pathUnder(path, "/marketplace-admin"):
		return role == domainidentity.RoleMarketplaceAdmin && !adminRestrictedPath(strings.TrimPrefix(path, "/marketplace-admin"))
	case pathUnder(path, "/super-admin"):
		return role == domainidentity.RoleSuperAdmin
	case pathUnder(path, "/finance-portal"):
		return role == domainidentity.RoleFinanceOperator && financeLegacyPath(strings.TrimPrefix(path, "/finance-portal"))
	case pathUnder(path, "/operations-portal"):
		return role == domainidentity.RoleOperations && operationsLegacyPath(strings.TrimPrefix(path, "/operations-portal"))
	case pathUnder(path, "/seller-admin"):
		return role == domainidentity.RoleSellerOwner || role == domainidentity.RoleSellerStaff
	case pathUnder(path, "/support-portal"):
		return role == domainidentity.RoleSupportAgent
	default:
		return false
	}
}

func adminRestrictedPath(path string) bool {
	path = strings.Trim(path, "/")
	return path == "security" || path == "roles" || path == "finance" ||
		strings.HasPrefix(path, "security/") || strings.HasPrefix(path, "roles/") ||
		strings.HasPrefix(path, "finance/") || path == "payments" || path == "failed-payments" ||
		path == "refunds" || path == "finance-reconciliation" || path == "settlements" ||
		path == "commission-rules"
}

func sellerPortalAllowed(path string, isOwner bool, permissions []string) bool {
	relative := strings.Trim(strings.TrimPrefix(path, "/seller-admin"), "/")
	if relative == "" {
		return true
	}
	if isOwner {
		switch relative {
		case "products", "products/new", "inventory", "orders", "returns", "earnings", "analytics", "team", "settings", "onboarding", "support":
			return true
		}
		if isSellerProductEditPath(relative) {
			return true
		}
		return false
	}

	permission := ""
	switch {
	case relative == "products":
		permission = domaincommerce.SellerPermissionProductRead
	case relative == "inventory":
		permission = domaincommerce.SellerPermissionInventoryRead
	case relative == "orders":
		permission = domaincommerce.SellerPermissionOrderRead
	case relative == "returns":
		permission = domaincommerce.SellerPermissionReturnRead
	case relative == "earnings" || relative == "commission" || relative == "analytics" || relative == "audit":
		permission = domaincommerce.SellerPermissionReportRead
	case relative == "onboarding":
		return true
	case isSellerProductEditPath(relative), relative == "products/new":
		permission = domaincommerce.SellerPermissionProductWrite
	case relative == "inventory/adjust":
		permission = domaincommerce.SellerPermissionInventoryWrite
	case strings.HasPrefix(relative, "orders/") && strings.HasSuffix(relative, "/fulfill"):
		permission = domaincommerce.SellerPermissionOrderFulfill
	case relative == "returns/status":
		permission = domaincommerce.SellerPermissionReturnProcess
	default:
		return false
	}
	for _, assigned := range permissions {
		if assigned == permission {
			return true
		}
	}
	return false
}

func isSellerProductEditPath(relative string) bool {
	if !strings.HasPrefix(relative, "products/") || !strings.HasSuffix(relative, "/edit") {
		return false
	}
	id := strings.TrimSuffix(strings.TrimPrefix(relative, "products/"), "/edit")
	if id == "" || strings.Contains(id, "/") {
		return false
	}
	parsed, err := strconv.ParseInt(id, 10, 64)
	return err == nil && parsed > 0
}

func pathUnder(path, prefix string) bool {
	return path == prefix || strings.HasPrefix(path, prefix+"/")
}

func legacyAdminDestination(path string, roles []domainidentity.Role) string {
	if path == "/admin" {
		workspaces := applicationauth.WorkspacesForRoles(roles)
		if len(workspaces) != 1 {
			return "/workspaces"
		}
		role := workspaces[0].Role
		if role == domainidentity.RoleMarketplaceAdmin || role == domainidentity.RoleSuperAdmin || role == domainidentity.RoleFinanceOperator || role == domainidentity.RoleOperations {
			return workspaces[0].Path
		}
		return ""
	}

	relative := strings.TrimPrefix(path, "/admin")
	if path == "/finance" {
		relative = ""
	}
	if !strings.HasPrefix(path, "/admin/") && path != "/finance" {
		return ""
	}

	for _, role := range roles {
		if role == domainidentity.RoleSuperAdmin && superAdminLegacyPath(relative) {
			return "/super-admin" + relative
		}
	}
	if financeLegacyPath(relative) {
		for _, role := range roles {
			if role == domainidentity.RoleFinanceOperator {
				if relative == "/finance" {
					relative = ""
				}
				return "/finance-portal" + relative
			}
		}
	}
	if operationsLegacyPath(relative) {
		for _, role := range roles {
			if role == domainidentity.RoleOperations {
				return "/operations-portal" + relative
			}
		}
	}
	for _, role := range roles {
		if role == domainidentity.RoleMarketplaceAdmin && !adminRestrictedPath(relative) {
			return "/marketplace-admin" + relative
		}
	}
	return ""
}

func financeLegacyPath(path string) bool {
	path = strings.Trim(path, "/")
	if path == "" || path == "finance" || path == "payments" || path == "failed-payments" || path == "refunds" || path == "finance-reconciliation" || path == "settlements" || path == "commission-rules" {
		return true
	}
	return false
}

func operationsLegacyPath(path string) bool {
	path = strings.Trim(path, "/")
	return path == "" || path == "orders" || path == "returns" || path == "refunds" || path == "disputes"
}

func superAdminLegacyPath(path string) bool {
	return true
}

func canonicalAdminPortal(path string) (domainidentity.Role, string, bool) {
	for _, portal := range []struct {
		prefix string
		role   domainidentity.Role
	}{
		{prefix: "/marketplace-admin", role: domainidentity.RoleMarketplaceAdmin},
		{prefix: "/super-admin", role: domainidentity.RoleSuperAdmin},
		{prefix: "/finance-portal", role: domainidentity.RoleFinanceOperator},
		{prefix: "/operations-portal", role: domainidentity.RoleOperations},
	} {
		if pathUnder(path, portal.prefix) {
			return portal.role, portal.prefix, true
		}
	}
	return "", "", false
}

func isCanonicalAdminPortal(path string) bool {
	_, _, ok := canonicalAdminPortal(path)
	return ok
}

func adminLegacyPagePath(path, root string) string {
	relative := strings.TrimPrefix(path, root)
	switch root {
	case "/marketplace-admin", "/super-admin":
		if relative == "" {
			return "/admin"
		}
		return "/admin" + relative
	case "/finance-portal":
		if relative == "" || relative == "/finance" {
			return "/admin/finance"
		}
		return "/admin" + relative
	case "/operations-portal":
		if relative == "" {
			return "/admin/returns"
		}
		return "/admin" + relative
	default:
		return "/admin"
	}
}

func canonicalAdminMutationPath(path string) string {
	_, root, ok := canonicalAdminPortal(path)
	if !ok {
		return ""
	}
	return adminLegacyPagePath(path, root)
}

func canonicalSellerMutationPath(path string) string {
	switch path {
	case "/seller-admin/products":
		return "/seller/products"
	case "/seller-admin/products/submit":
		return "/seller/products/submit"
	case "/seller-admin/inventory/adjust":
		return "/seller/inventory/adjust"
	case "/seller-admin/orders/fulfill":
		return "/seller/orders/fulfill"
	case "/seller-admin/team/add":
		return "/seller/team/add"
	case "/seller-admin/team/remove":
		return "/seller/team/remove"
	default:
		if strings.HasPrefix(path, "/seller-admin/products/") && strings.HasSuffix(path, "/edit") {
			id := strings.TrimSuffix(strings.TrimPrefix(path, "/seller-admin/products/"), "/edit")
			if id != "" && !strings.Contains(id, "/") {
				if _, err := strconv.ParseInt(id, 10, 64); err == nil {
					return "/seller/products/" + id + "/edit"
				}
			}
		}
		return ""
	}
}

func sellerMutationAllowed(path string, isOwner bool, permissions []string) bool {
	if isOwner {
		return true
	}
	permission := ""
	switch {
	case path == "/seller-admin/products" || path == "/seller-admin/products/submit" || strings.HasPrefix(path, "/seller-admin/products/") && strings.HasSuffix(path, "/edit"):
		permission = domaincommerce.SellerPermissionProductWrite
	case path == "/seller-admin/inventory/adjust":
		permission = domaincommerce.SellerPermissionInventoryWrite
	case path == "/seller-admin/orders/fulfill":
		permission = domaincommerce.SellerPermissionOrderFulfill
	default:
		return false
	}
	return containsPermission(permissions, permission)
}

func containsPermission(permissions []string, required string) bool {
	for _, permission := range permissions {
		if permission == required {
			return true
		}
	}
	return false
}

func roleString(role domainidentity.Role) string {
	return string(role)
}

func roleLabel(role domainidentity.Role) string {
	switch role {
	case domainidentity.RoleMarketplaceAdmin:
		return "Marketplace Admin"
	case domainidentity.RoleSuperAdmin:
		return "Super Admin"
	case domainidentity.RoleFinanceOperator:
		return "Finance Operator"
	case domainidentity.RoleOperations:
		return "Returns Operations"
	case domainidentity.RoleSellerOwner:
		return "Seller Owner"
	case domainidentity.RoleSellerStaff:
		return "Seller Staff"
	case domainidentity.RoleSupportAgent:
		return "Support Agent"
	case domainidentity.RoleCustomer:
		return "Customer"
	default:
		return "Account"
	}
}

func portalLabel(role domainidentity.Role) string {
	switch role {
	case domainidentity.RoleMarketplaceAdmin:
		return "Marketplace Portal"
	case domainidentity.RoleSuperAdmin:
		return "Super Admin Portal"
	case domainidentity.RoleFinanceOperator:
		return "Finance Portal"
	case domainidentity.RoleOperations:
		return "Operations Portal"
	default:
		return "Workspace"
	}
}

func hasIdentityRole(roles []domainidentity.Role, wanted domainidentity.Role) bool {
	for _, role := range roles {
		if role == wanted {
			return true
		}
	}
	return false
}

func customerRoleAllowed(roles []domainidentity.Role) bool {
	return hasIdentityRole(roles, domainidentity.RoleCustomer)
}

func requestWithPath(r *http.Request, path string) *http.Request {
	cloned := r.Clone(r.Context())
	urlCopy := *r.URL
	urlCopy.Path = path
	cloned.URL = &urlCopy
	return cloned
}

func redirectToLogin(w http.ResponseWriter, r *http.Request) {
	target := "/login?next=" + url.QueryEscape(r.URL.RequestURI())
	http.Redirect(w, r, target, http.StatusSeeOther)
}

func legacySellerPortalDestination(path string) string {
	if path == "/seller" {
		return "/seller-admin"
	}
	relative := strings.Trim(strings.TrimPrefix(path, "/seller/"), "/")
	destinations := map[string]string{
		"products": "/seller-admin/products", "products/new": "/seller-admin/products/new",
		"orders": "/seller-admin/orders", "inventory": "/seller-admin/inventory",
		"returns": "/seller-admin/returns", "earnings": "/seller-admin/earnings",
		"analytics": "/seller-admin/analytics", "audit": "/seller-admin/audit",
		"team": "/seller-admin/team", "settings": "/seller-admin/settings",
		"onboarding": "/seller-admin/onboarding", "support": "/seller-admin/support",
		"commission": "/seller-admin/commission",
	}
	if destination := destinations[relative]; destination != "" {
		return destination
	}
	if strings.HasPrefix(relative, "products/") && strings.HasSuffix(relative, "/edit") {
		id := strings.TrimSuffix(strings.TrimPrefix(relative, "products/"), "/edit")
		if isSellerProductEditPath("products/" + id + "/edit") {
			return "/seller-admin/products/" + id + "/edit"
		}
	}
	return ""
}

func legacySupportPortalDestination(path string) string {
	if path == "/support" || path == "/support/" {
		return "/support-portal"
	}
	switch strings.Trim(strings.TrimPrefix(path, "/support/"), "/") {
	case "cases", "tickets":
		return "/support-portal/cases"
	case "customers":
		return "/support-portal/customers"
	case "orders":
		return "/support-portal/orders"
	default:
		return ""
	}
}

func redirectPortalDestination(w http.ResponseWriter, r *http.Request, destination string) {
	if r.URL.RawQuery != "" {
		destination += "?" + r.URL.RawQuery
	}
	http.Redirect(w, r, destination, http.StatusSeeOther)
}
