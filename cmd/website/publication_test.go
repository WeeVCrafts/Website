package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wecratfs/commerce/internal/config"
)

func TestProductionWithholdsUnapprovedInformationAndNewCheckout(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	cfg := config.Config{Environment: "production"}
	for _, path := range []string{"/contact", "/privacy", "/shipping", "/terms", "/pages/returns", "/seller-admin/terms", "/checkout", "/api/v1/checkout", "/api/v1/checkout/intents"} {
		w := httptest.NewRecorder()
		releaseAccess(next, cfg).ServeHTTP(w, httptest.NewRequest(http.MethodPost, path, nil))
		if w.Code != 503 || w.Header().Get("Cache-Control") != "private, no-store" || !strings.Contains(w.Header().Get("X-Robots-Tag"), "noindex") {
			t.Errorf("unapproved %s status=%d headers=%v", path, w.Code, w.Header())
		}
	}
	for _, path := range []string{"/", "/products", "/login", "/reset-password", "/orders/EXISTING", "/api/v1/orders/EXISTING/payment", "/webhooks/razorpay", "/health/ready"} {
		w := httptest.NewRecorder()
		releaseAccess(next, cfg).ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != 204 {
			t.Errorf("publication gate blocked existing workflow %s: %d", path, w.Code)
		}
	}
	cfg.PublicContentApproved = true
	w := httptest.NewRecorder()
	releaseAccess(next, cfg).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/privacy", nil))
	if w.Code != 204 {
		t.Fatal("explicitly approved content unavailable")
	}
	cfg.PublicContentApproved = false
	cfg.Environment = "development"
	w = httptest.NewRecorder()
	releaseAccess(next, cfg).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/contact", nil))
	if w.Code != 204 {
		t.Fatal("isolated development previews blocked")
	}
}
