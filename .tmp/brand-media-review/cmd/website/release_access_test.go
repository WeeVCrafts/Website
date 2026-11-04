package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wecratfs/commerce/internal/config"
)

func TestStagingAccess(t *testing.T) {
	cfg := config.Config{Environment: "production", Staging: true, StagingUsername: "qa", StagingPassword: "dedicated-staging-password-123"}
	h := releaseAccess(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }), cfg)
	for _, tc := range []struct {
		path          string
		authenticated bool
		status        int
	}{
		{"/", false, 401}, {"/assets/test.css", false, 401}, {"/", true, 204},
		{"/health/live", false, 204}, {"/health/ready", false, 204},
		{"/webhooks/razorpay", false, 204}, {"/health/metrics", true, 404},
	} {
		r := httptest.NewRequest("GET", tc.path, nil)
		if tc.authenticated {
			r.SetBasicAuth(cfg.StagingUsername, cfg.StagingPassword)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != tc.status {
			t.Fatalf("%s: got %d want %d", tc.path, w.Code, tc.status)
		}
		if w.Header().Get("X-Robots-Tag") != "noindex, nofollow" {
			t.Fatal("staging is indexable")
		}
	}
}

func TestProductionMetricsRequireDedicatedToken(t *testing.T) {
	cfg := config.Config{Environment: "production", OperationalMetricsToken: "dedicated-operational-token-32-characters"}
	h := releaseAccess(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }), cfg)
	for _, token := range []string{"", "Bearer wrong", "Bearer " + cfg.OperationalMetricsToken} {
		r := httptest.NewRequest("GET", "/health/metrics", nil)
		r.Header.Set("Authorization", token)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		want := 404
		if token == "Bearer "+cfg.OperationalMetricsToken {
			want = 204
		}
		if w.Code != want {
			t.Fatalf("metrics status=%d want=%d", w.Code, want)
		}
	}
}
