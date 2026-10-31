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
					w.Header().Set("WWW-Authenticate", `Basic realm="WeevCrafts staging", charset="UTF-8"`)
					http.Error(w, "Staging authentication required", http.StatusUnauthorized)
					return
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}
