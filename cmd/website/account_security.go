package main

import (
	"encoding/json"
	applicationauth "github.com/wecratfs/commerce/internal/application/auth"
	domainidentity "github.com/wecratfs/commerce/internal/domain/identity"
	"net/http"
	"strings"
)

// Every protected transport, including legacy routes and local APIs, shares
// this guard. Page navigation is never the authorization boundary.
func accountSecurity(next http.Handler, sessions *applicationauth.SessionService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "SAMEORIGIN")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		if cookie, err := r.Cookie("wecratfs_session"); err == nil {
			w.Header().Set("Cache-Control", "private, no-store")
			session, err := sessions.Get(r.Context(), cookie.Value)
			if err == nil {
				roles := make([]domainidentity.Role, 0, len(session.RoleSlugs))
				for _, role := range session.RoleSlugs {
					roles = append(roles, domainidentity.Role(role))
				}
				if applicationauth.LoginDestination(roles) == "/access-unavailable" && protectedIdentityPath(r.URL.Path) {
					if strings.HasPrefix(r.URL.Path, "/api/") {
						w.Header().Set("Content-Type", "application/json")
						w.WriteHeader(403)
						_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": "ROLE_ASSIGNMENT_REQUIRED", "message": "Ask Super Admin to assign one role to your account."}})
					} else {
						http.Redirect(w, r, "/access-unavailable", http.StatusSeeOther)
					}
					return
				}
			}
		}
		if protectedIdentityPath(r.URL.Path) || strings.HasPrefix(r.URL.Path, "/api/v1/auth/") {
			w.Header().Set("Cache-Control", "private, no-store")
		}
		next.ServeHTTP(w, r)
	})
}

func protectedIdentityPath(path string) bool {
	if path == "/logout" || path == "/api/v1/auth/logout" {
		return false
	}
	if strings.HasPrefix(path, "/api/") {
		for _, public := range []string{"/api/v1/products", "/api/v1/search", "/api/v1/auth"} {
			if path == public || strings.HasPrefix(path, public+"/") {
				return false
			}
		}
		return true
	}
	for _, prefix := range []string{"/account", "/orders", "/returns", "/tracking", "/payments", "/checkout", "/wishlist", "/cart", "/ui/wishlist", "/ui/cart", "/seller", "/seller-admin", "/admin", "/super-admin", "/marketplace-admin", "/finance", "/finance-portal", "/operations-portal", "/support", "/support-portal", "/privacy/center", "/privacy/consent", "/privacy/requests", "/privacy/delete", "/dev"} {
		if path == prefix || strings.HasPrefix(path, prefix+"/") {
			return true
		}
	}
	return false
}
