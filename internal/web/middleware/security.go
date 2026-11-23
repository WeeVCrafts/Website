package middleware

import (
	"net/http"
)

type SecurityOptions struct {
	IsProduction bool
}

// EnterpriseSecurityHeaders applies defense-in-depth HTTP security headers to all responses.
func EnterpriseSecurityHeaders(opts SecurityOptions) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Clickjacking & framing defense (SAMEORIGIN allows admin storefront preview frames)
			w.Header().Set("X-Frame-Options", "SAMEORIGIN")
			// Prevent MIME sniffing
			w.Header().Set("X-Content-Type-Options", "nosniff")
			// Referrer policy
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
			// Permissions policy
			w.Header().Set("Permissions-Policy", "geolocation=(), camera=(), microphone=(), payment=(self)")

			// Strict Transport Security (HSTS) on HTTPS / Production
			if opts.IsProduction || r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
				w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")
			}

			next.ServeHTTP(w, r)
		})
	}
}
