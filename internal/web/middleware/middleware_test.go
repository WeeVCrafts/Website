package middleware

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRecovery_HandlesPanicHTML(t *testing.T) {
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logBuf, nil))

	failingHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("deliberate test panic in handler")
	})

	wrapped := Recovery(logger)(failingHandler)

	req := httptest.NewRequest(http.MethodGet, "/products", nil)
	req.Header.Set("X-Request-ID", "req-test-123")
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", rec.Code)
	}

	if !bytes.Contains(rec.Body.Bytes(), []byte("Something went wrong")) {
		t.Errorf("expected HTML body with error message, got %s", rec.Body.String())
	}

	if !bytes.Contains(logBuf.Bytes(), []byte("deliberate test panic in handler")) {
		t.Errorf("expected log to contain panic message, got %s", logBuf.String())
	}
}

func TestRecovery_HandlesPanicJSON(t *testing.T) {
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logBuf, nil))

	failingHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("deliberate json panic")
	})

	wrapped := Recovery(logger)(failingHandler)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/checkout", nil)
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", rec.Code)
	}

	var res map[string]map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&res); err != nil {
		t.Fatalf("expected valid JSON response: %v", err)
	}
	if res["error"]["code"] != "INTERNAL_SERVER_ERROR" {
		t.Errorf("expected code INTERNAL_SERVER_ERROR, got %s", res["error"]["code"])
	}
}

func TestEnterpriseSecurityHeaders(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := EnterpriseSecurityHeaders(SecurityOptions{IsProduction: true})(handler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	headers := rec.Header()
	if headers.Get("X-Frame-Options") != "SAMEORIGIN" {
		t.Errorf("expected X-Frame-Options: SAMEORIGIN, got %s", headers.Get("X-Frame-Options"))
	}
	if headers.Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("expected X-Content-Type-Options: nosniff, got %s", headers.Get("X-Content-Type-Options"))
	}
	if headers.Get("Referrer-Policy") != "strict-origin-when-cross-origin" {
		t.Errorf("expected Referrer-Policy: strict-origin-when-cross-origin, got %s", headers.Get("Referrer-Policy"))
	}
	if headers.Get("Permissions-Policy") != "geolocation=(), camera=(), microphone=(), payment=(self)" {
		t.Errorf("expected Permissions-Policy set, got %s", headers.Get("Permissions-Policy"))
	}
	if headers.Get("Strict-Transport-Security") != "max-age=31536000; includeSubDomains; preload" {
		t.Errorf("expected HSTS set, got %s", headers.Get("Strict-Transport-Security"))
	}
}
