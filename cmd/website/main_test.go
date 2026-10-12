package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/wecratfs/commerce/internal/config"
)

func TestDevelopmentPaymentHelpersRequireLocalDevelopmentRuntime(t *testing.T) {
	for _, tc := range []struct {
		name string
		cfg  config.Config
		want bool
	}{
		{"local development", config.Config{Environment: "development", LocalMode: true, PaymentMode: "local"}, true},
		{"production local provider", config.Config{Environment: "production", LocalMode: true, PaymentMode: "local"}, false},
		{"production provider", config.Config{Environment: "production", LocalMode: false, PaymentMode: "razorpay"}, false},
		{"local mode off", config.Config{Environment: "development", LocalMode: false, PaymentMode: "local"}, false},
		{"real provider in development", config.Config{Environment: "development", LocalMode: true, PaymentMode: "razorpay"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mux := http.NewServeMux()
			if developmentPaymentHelpersEnabled(tc.cfg) {
				mux.HandleFunc("/dev/payments/", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
			}
			recorder := httptest.NewRecorder()
			mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/dev/payments/owned-order/capture", nil))
			wantStatus := http.StatusNotFound
			if tc.want {
				wantStatus = http.StatusNoContent
			}
			if recorder.Code != wantStatus {
				t.Fatalf("development payment route status = %d, want %d", recorder.Code, wantStatus)
			}
		})
	}
}

type fakeRateLimiter struct {
	allowed bool
	err     error
	keys    []string
}

func TestDemoCaptureOnlyAvailableForConfiguredDemoProvider(t *testing.T) {
	for _, mode := range []string{"local", "razorpay"} {
		mux := http.NewServeMux()
		calls := 0
		registerDemoPaymentCapture(mux, config.Config{Environment: "production", PaymentMode: mode}, func(w http.ResponseWriter, r *http.Request) {
			calls++
			if r.URL.Path != "/dev/payments/owned-order/capture" || r.Header.Get("X-CSRF-Token") != "test-token" || r.Method != http.MethodPost {
				t.Fatalf("capture did not preserve authentication request: %s", r.URL.Path)
			}
			w.WriteHeader(http.StatusNoContent)
		})
		r := httptest.NewRequest(http.MethodPost, "/api/v1/payments/demo-capture/owned-order", nil)
		r.Header.Set("X-CSRF-Token", "test-token")
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		if mode == "local" && (w.Code != 204 || calls != 1) {
			t.Fatal("production demo checkout is unavailable")
		}
		if mode != "local" && (w.Code != 404 || calls != 0) {
			t.Fatal("demo capture exposed for real payment provider")
		}
		for _, path := range []string{"/api/v1/payments/demo-capture/owned-order/fail", "/dev/payments/owned-order/fail", "/api/v1/payments/demo-capture/"} {
			w = httptest.NewRecorder()
			mux.ServeHTTP(w, httptest.NewRequest(http.MethodPost, path, nil))
			if w.Code != 404 {
				t.Fatalf("invalid demo route exposed: %s", path)
			}
		}
	}
}

func (f *fakeRateLimiter) Allow(_ context.Context, key string, _ int, _ time.Duration) (bool, error) {
	f.keys = append(f.keys, key)
	return f.allowed, f.err
}

func TestAuthRateLimitProtectsOnlyAuthenticationRoutes(t *testing.T) {
	limiter := &fakeRateLimiter{allowed: false}
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	handler := authRateLimit(next, limiter)

	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{}`))
	request.RemoteAddr = "192.0.2.10:4000"
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("expected auth request to be rate limited, got %d", recorder.Code)
	}
	if recorder.Header().Get("Retry-After") != "60" || !strings.Contains(recorder.Body.String(), `"RATE_LIMITED"`) {
		t.Fatalf("expected retry/error contract, headers=%v body=%s", recorder.Header(), recorder.Body.String())
	}
	if len(limiter.keys) != 1 || !strings.HasPrefix(limiter.keys[0], "auth:192.0.2.10") {
		t.Fatalf("unexpected limiter key: %v", limiter.keys)
	}

	limiter.allowed = true
	request = httptest.NewRequest(http.MethodGet, "/api/v1/products", nil)
	request.RemoteAddr = "192.0.2.10:4000"
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("catalog request should bypass auth limiter, got %d", recorder.Code)
	}
	if len(limiter.keys) != 1 {
		t.Fatalf("catalog request unexpectedly consumed auth limit: %v", limiter.keys)
	}

	request = httptest.NewRequest(http.MethodGet, "/login", nil)
	request.RemoteAddr = "192.0.2.10:4000"
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("login form should remain available when the attempt limit is reached, got %d", recorder.Code)
	}
	if len(limiter.keys) != 1 {
		t.Fatalf("opening the login form unexpectedly consumed auth limit: %v", limiter.keys)
	}
}

func TestAuthRateLimitDoesNotCountSessionRefreshOrLogout(t *testing.T) {
	limiter := &fakeRateLimiter{allowed: false}
	handler := authRateLimit(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}), limiter)
	for _, path := range []string{"/api/v1/auth/session", "/api/v1/auth/logout", "/api/v1/products"} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		request.RemoteAddr = "192.0.2.12:4000"
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusNoContent {
			t.Errorf("%s should bypass auth-attempt limiting, got %d", path, recorder.Code)
		}
	}
	if len(limiter.keys) != 0 {
		t.Fatalf("session refresh, logout, or catalog requests consumed auth limit: %v", limiter.keys)
	}
}

func TestAuthRateLimitUnavailableReturnsJSONServiceError(t *testing.T) {
	limiter := &fakeRateLimiter{err: context.DeadlineExceeded}
	handler := authRateLimit(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}), limiter)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(`{}`))
	request.RemoteAddr = "192.0.2.11:4000"
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected limiter outage status 503, got %d", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), `"RATE_LIMIT_UNAVAILABLE"`) {
		t.Fatalf("expected structured outage error, got %s", recorder.Body.String())
	}
}

func TestAuthRateLimitsUseSeparateBucketsForIndependentAccountActions(t *testing.T) {
	limiter := &fakeRateLimiter{allowed: true}
	handler := authRateLimit(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}), limiter)
	for _, path := range []string{
		"/api/v1/auth/login",
		"/api/v1/auth/register",
		"/api/v1/auth/password-reset/request",
		"/api/v1/auth/email-verification/confirm",
		"/account/verify-email",
	} {
		request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{}`))
		request.RemoteAddr = "192.0.2.13:4000"
		handler.ServeHTTP(httptest.NewRecorder(), request)
	}
	want := []string{
		"auth:192.0.2.13:login",
		"auth:192.0.2.13:register",
		"auth:192.0.2.13:password-recovery",
		"auth:192.0.2.13:email-verification",
		"auth:192.0.2.13:email-verification",
	}
	if len(limiter.keys) != len(want) {
		t.Fatalf("expected one limiter check per protected operation, got %v", limiter.keys)
	}
	for index := range want {
		if limiter.keys[index] != want[index] {
			t.Errorf("limiter key %d = %q, want %q", index, limiter.keys[index], want[index])
		}
	}
}
