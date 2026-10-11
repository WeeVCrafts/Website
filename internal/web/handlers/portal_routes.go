package handlers

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"

	applicationauth "github.com/wecratfs/commerce/internal/application/auth"
	domainidentity "github.com/wecratfs/commerce/internal/domain/identity"
)

func roleAllowsPortalPath(role domainidentity.Role, path string) bool {
	if pathUnder(path, "/super-admin") {
		return role == domainidentity.RoleSuperAdmin
	}
	if pathUnder(path, "/admin") {
		return domainidentity.CompanyRole(role) && (!adminRestrictedPath(strings.TrimPrefix(path, "/admin")) || role == domainidentity.RoleSuperAdmin)
	}
	if pathUnder(path, "/seller-admin") {
		return role == domainidentity.RoleSeller
	}
	return false
}
func adminRestrictedPath(path string) bool {
	path = strings.Trim(path, "/")
	for _, prefix := range []string{"security", "roles", "users", "settings", "audit-logs", "security-events", "system-health", "search-indexing-health", "worker-queue-health"} {
		if path == prefix || strings.HasPrefix(path, prefix+"/") {
			return true
		}
	}
	return false
}
func sellerPortalAllowed(path string, isOwner bool, permissions []string) bool {
	if !isOwner {
		return false
	}
	relative := strings.Trim(strings.TrimPrefix(path, "/seller-admin"), "/")
	switch relative {
	case "", "products", "products/new", "inventory", "orders", "returns", "earnings", "analytics", "settings", "onboarding", "support", "terms":
		return true
	}
	return isSellerProductEditPath(relative) || sellerOrderDetailPath(relative)
}

func sellerOrderDetailPath(relative string) bool {
	if !strings.HasPrefix(relative, "orders/") {
		return false
	}
	number := strings.TrimPrefix(relative, "orders/")
	return number != "" && !strings.Contains(number, "/") && number != "fulfill"
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

// Retired GET portal links are resolved only after current authorization.
func legacyAdminDestination(path string, roles []domainidentity.Role) string {
	destination := applicationauth.LoginDestination(roles)
	if destination != "/admin" && destination != "/super-admin" {
		return ""
	}
	relative := ""
	for _, root := range []string{"/marketplace-admin", "/finance-portal", "/operations-portal", "/admin", "/finance"} {
		if pathUnder(path, root) {
			relative = strings.TrimPrefix(path, root)
			break
		}
	}
	if path == "/finance" {
		relative = "/finance"
	}
	if relative == "" && pathUnder(path, "/finance-portal") {
		relative = "/finance"
	}
	if relative == "" && pathUnder(path, "/operations-portal") {
		relative = "/returns"
	}
	if !knownAdminPage(relative) || !roleAllowsPortalPath(roles[0], destination+relative) {
		return ""
	}
	return destination + relative
}
func financeLegacyPath(path string) bool    { return knownAdminPage(path) }
func operationsLegacyPath(path string) bool { return knownAdminPage(path) }
func superAdminLegacyPath(path string) bool { return knownAdminPage(path) }

func knownAdminPage(path string) bool {
	switch strings.Trim(path, "/") {
	case "", "dashboard", "sellers", "products", "inventory", "orders", "returns", "finance", "support", "marketing", "analytics", "security",
		"brands", "customers", "category-management", "commission-rules", "settings", "reviews", "payments", "failed-payments", "refunds", "settlements", "finance-reconciliation",
		"roles", "users", "audit-logs", "security-events", "system-health", "search-indexing-health", "worker-queue-health", "seller-approvals", "seller-suspensions":
		return true
	}
	return false
}

func canonicalAdminPortal(path string) (domainidentity.Role, string, bool) {
	if pathUnder(path, "/super-admin") {
		return domainidentity.RoleSuperAdmin, "/super-admin", true
	}
	if pathUnder(path, "/admin") {
		return domainidentity.RoleAdmin, "/admin", true
	}
	return "", "", false
}

func isCanonicalAdminPortal(path string) bool {
	_, _, ok := canonicalAdminPortal(path)
	return ok
}

func adminLegacyPagePath(path, root string) string { return "/admin" + strings.TrimPrefix(path, root) }

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
	case "/seller-admin/products/archive":
		return "/seller/products/archive"
	case "/seller-admin/products/restore":
		return "/seller/products/restore"
	case "/seller-admin/products/variants/create":
		return "/seller/products/variants/create"
	case "/seller-admin/products/variants/update":
		return "/seller/products/variants/update"
	case "/seller-admin/products/variants/archive":
		return "/seller/products/variants/archive"
	case "/seller-admin/inventory/adjust":
		return "/seller/inventory/adjust"
	case "/seller-admin/orders/fulfill":
		return "/seller/orders/fulfill"
	case "/seller-admin/onboarding":
		return "/seller/onboard"
	case "/seller-admin/settings":
		return "/seller/settings/update"
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
	return isOwner && !strings.Contains(path, "/team")
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
	case domainidentity.RoleAdmin:
		return "Admin"
	case domainidentity.RoleSuperAdmin:
		return "Super Admin"
	case domainidentity.RoleSeller:
		return "Seller"
	case domainidentity.RoleCustomer:
		return "Customer"
	}
	return "Account"
}
func portalLabel(role domainidentity.Role) string {
	if role == domainidentity.RoleSuperAdmin {
		return "Super Admin Portal"
	}
	return "Admin Portal"
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
	return len(roles) == 1 && roles[0] == domainidentity.RoleCustomer
}

func requestWithPath(r *http.Request, path string) *http.Request {
	cloned := r.Clone(r.Context())
	urlCopy := *r.URL
	urlCopy.Path = path
	cloned.URL = &urlCopy
	cloned.MultipartForm = r.MultipartForm
	cloned.Form = r.Form
	cloned.PostForm = r.PostForm
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
		"settings":   "/seller-admin/settings",
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
