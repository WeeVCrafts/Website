package main

import (
	"crypto/sha256"
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/wecratfs/commerce/internal/config"
)

// releaseAccess keeps staging private while allowing provider callbacks and
// minimal infrastructure probes. Callback handlers verify provider signatures.
func releaseAccess(next http.Handler, cfg config.Config) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.EqualFold(cfg.Environment, "production") && r.URL.Path == "/health/metrics" {
			w.Header().Set("Cache-Control", "private, no-store")
			w.Header().Set("X-Robots-Tag", "noindex, nofollow")
			provided := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			actual, expected := sha256.Sum256([]byte(provided)), sha256.Sum256([]byte(cfg.OperationalMetricsToken))
			if len(cfg.OperationalMetricsToken) < 32 || !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") || subtle.ConstantTimeCompare(actual[:], expected[:]) != 1 {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Cache-Control", "private, no-store")
			w.Header().Set("X-Robots-Tag", "noindex, nofollow")
			next.ServeHTTP(w, r)
			return
		}
		if cfg.Staging {
			w.Header().Set("X-Robots-Tag", "noindex, nofollow")
			w.Header().Set("Cache-Control", "private, no-store")
			if r.URL.Path != "/health/live" && r.URL.Path != "/health/ready" && r.URL.Path != "/webhooks/razorpay" {
				user, password, ok := r.BasicAuth()
				actualUser, actualPassword := sha256.Sum256([]byte(user)), sha256.Sum256([]byte(password))
				expectedUser, expectedPassword := sha256.Sum256([]byte(cfg.StagingUsername)), sha256.Sum256([]byte(cfg.StagingPassword))
				if !ok || subtle.ConstantTimeCompare(actualUser[:], expectedUser[:]) != 1 || subtle.ConstantTimeCompare(actualPassword[:], expectedPassword[:]) != 1 {
					w.Header().Set("WWW-Authenticate", `Basic realm="Weevcrafts staging", charset="UTF-8"`)
					http.Error(w, "Staging authentication required", http.StatusUnauthorized)
					return
				}
			}
		}
		if strings.EqualFold(cfg.Environment, "production") && !cfg.PublicContentApproved && requiresPublishedContent(r.URL.Path) {
			w.Header().Set("Cache-Control", "private, no-store")
			w.Header().Set("X-Robots-Tag", "noindex, nofollow")
			w.Header().Set("Retry-After", "3600")
			if strings.HasPrefix(r.URL.Path, "/api/") {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusServiceUnavailable)
				_, _ = w.Write([]byte(`{"error":{"code":"PUBLICATION_PENDING","message":"Checkout is unavailable while marketplace information awaits publication."}}`))
			} else {
				http.Error(w, "Marketplace information is awaiting publication. Please try again later.", http.StatusServiceUnavailable)
			}
			return
		}
		if !cfg.CheckoutEnabled && isNewCheckout(r.URL.Path) {
			w.Header().Set("Cache-Control", "private, no-store")
			w.Header().Set("X-Robots-Tag", "noindex, nofollow")
			w.Header().Set("Retry-After", "3600")
			if strings.HasPrefix(r.URL.Path, "/api/") {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusServiceUnavailable)
				_, _ = w.Write([]byte(`{"error":{"code":"CHECKOUT_UNAVAILABLE","message":"New checkout is currently unavailable. Please try again later."}}`))
			} else {
				http.Error(w, "New checkout is currently unavailable. Please try again later.", http.StatusServiceUnavailable)
			}
			return
		}
		next.ServeHTTP(w, r)
	})
}

// Existing order payment/recovery and signed callbacks do not create a new
// marketplace checkout and must remain reachable while this release gate is off.
func isNewCheckout(path string) bool {
	return path == "/checkout" || strings.HasPrefix(path, "/checkout/") || path == "/api/v1/checkout" || strings.HasPrefix(path, "/api/v1/checkout/")
}

// Withhold unapproved information on both current and legacy paths. Keep
// existing orders, provider callbacks and account recovery available.
func requiresPublishedContent(path string) bool {
	if strings.HasPrefix(path, "/pages/") {
		return true
	}
	switch path {
	case "/about", "/our-story", "/sustainability", "/press", "/careers", "/contact", "/faq", "/shipping", "/returns-policy", "/size-guide", "/terms", "/privacy", "/seller-admin/terms", "/checkout", "/api/v1/checkout", "/api/v1/checkout/intents", "/api/v1/checkout/no-payment":
		return true
	}
	return false
}
