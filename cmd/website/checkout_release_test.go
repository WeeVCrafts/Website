package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	metricsadapter "github.com/wecratfs/commerce/internal/adapters/metrics"
	"github.com/wecratfs/commerce/internal/config"
	healthTransport "github.com/wecratfs/commerce/internal/transport/health"
)

func TestApprovedInformationDoesNotOpenCheckout(t *testing.T) {
	calls := 0
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { calls++; w.WriteHeader(204) })
	cfg := config.Config{Environment: "production", PublicContentApproved: true}
	for _, method := range []string{"GET", "POST"} {
		for _, path := range []string{"/checkout", "/api/v1/checkout", "/api/v1/checkout/intents", "/api/v1/checkout/no-payment"} {
			w := httptest.NewRecorder()
			releaseAccess(next, cfg).ServeHTTP(w, httptest.NewRequest(method, path, nil))
			if w.Code != 503 || w.Header().Get("Cache-Control") != "private, no-store" || !strings.Contains(w.Header().Get("X-Robots-Tag"), "noindex") {
				t.Fatalf("%s %s status=%d headers=%v", method, path, w.Code, w.Header())
			}
			if strings.HasPrefix(path, "/api/") && !strings.Contains(w.Body.String(), `"code":"CHECKOUT_UNAVAILABLE"`) {
				t.Fatal(w.Body.String())
			}
		}
	}
	if calls != 0 {
		t.Fatal("disabled checkout reached a mutating handler")
	}
	for _, path := range []string{"/privacy", "/contact", "/orders/EXISTING", "/api/v1/orders/EXISTING/payment", "/login", "/reset-password", "/webhooks/razorpay"} {
		w := httptest.NewRecorder()
		releaseAccess(next, cfg).ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 204 {
			t.Fatalf("existing workflow %s blocked: %d", path, w.Code)
		}
	}
}

func TestProtectedOperationalMetricsObserveGateWithoutPrivateData(t *testing.T) {
	cfg := config.Config{Environment: "production", PublicContentApproved: true, OperationalMetricsToken: "monitoring-fixture-token-for-tests-only"}
	m := metricsadapter.NewHTTPMetrics()
	h := healthTransport.NewHandlerWithMetrics(nil, func() any {
		return metricsadapter.OperationalSnapshot{SearchSnapshot: metricsadapter.SearchSnapshot{Observations: 7}, HTTP: m.Snapshot()}
	})
	mux := http.NewServeMux()
	mux.HandleFunc("/health/metrics", h.Metrics)
	mux.HandleFunc("/api/v1/checkout", func(http.ResponseWriter, *http.Request) { t.Fatal("closed checkout reached order handler") })
	server := m.Middleware(releaseAccess(mux, cfg))
	w := httptest.NewRecorder()
	server.ServeHTTP(w, httptest.NewRequest("POST", "/api/v1/checkout?token=PRIVATE_CANARY", nil))
	if w.Code != 503 {
		t.Fatal("checkout did not fail closed")
	}
	for _, authorized := range []bool{false, true} {
		r := httptest.NewRequest("GET", "/health/metrics", nil)
		if authorized {
			r.Header.Set("Authorization", "Bearer "+cfg.OperationalMetricsToken)
		}
		w := httptest.NewRecorder()
		server.ServeHTTP(w, r)
		if !authorized {
			if w.Code != 404 {
				t.Fatal("metrics exposed without token")
			}
			continue
		}
		if w.Code != 200 || strings.Contains(w.Body.String(), "PRIVATE_CANARY") || strings.Contains(w.Body.String(), cfg.OperationalMetricsToken) {
			t.Fatal("metrics response status or privacy failure")
		}
		var s metricsadapter.OperationalSnapshot
		if err := json.Unmarshal(w.Body.Bytes(), &s); err != nil {
			t.Fatal(err)
		}
		if s.Observations != 7 || s.HTTP.Requests != 1 || s.HTTP.WriteRequests != 1 || s.HTTP.Errors != 1 {
			t.Fatalf("unexpected metrics: %+v", s)
		}
	}
}

func TestExplicitCheckoutEnableRemainsSubjectToPublication(t *testing.T) {
	cfg := config.Config{Environment: "production", CheckoutEnabled: true, PublicContentApproved: true}
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(204) })
	for _, method := range []string{"GET", "POST"} {
		w := httptest.NewRecorder()
		releaseAccess(next, cfg).ServeHTTP(w, httptest.NewRequest(method, "/api/v1/checkout", nil))
		if w.Code != 204 {
			t.Fatalf("explicit checkout blocked: %d", w.Code)
		}
	}
	cfg.PublicContentApproved = false
	w := httptest.NewRecorder()
	releaseAccess(next, cfg).ServeHTTP(w, httptest.NewRequest("POST", "/api/v1/checkout", nil))
	if w.Code != 503 || !strings.Contains(w.Body.String(), "PUBLICATION_PENDING") {
		t.Fatal("checkout bypassed publication")
	}
}
