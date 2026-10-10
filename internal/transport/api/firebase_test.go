package api

import (
	"context"
	"errors"
	applicationauth "github.com/wecratfs/commerce/internal/application/auth"
	"github.com/wecratfs/commerce/internal/ports"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestFirebaseSessionRejectsMissingCSRFAndForeignOrigins(t *testing.T) {
	h := (&Handler{}).WithFirebase(&applicationauth.FirebaseService{})
	for _, tc := range []struct{ name, origin, csrf string }{{"missing csrf", "http://localhost:8080", ""}, {"foreign origin", "https://attacker.example", strings.Repeat("a", 43)}, {"missing origin", "", strings.Repeat("a", 43)}} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "http://localhost:8080/api/v1/auth/firebase/session", strings.NewReader(`{"id_token":"untrusted"}`))
			r.AddCookie(&http.Cookie{Name: "wecratfs_csrf", Value: strings.Repeat("a", 43)})
			r.Header.Set("Origin", tc.origin)
			r.Header.Set("X-CSRF-Token", tc.csrf)
			w := httptest.NewRecorder()
			h.FirebaseSession(w, r)
			if w.Code != 403 {
				t.Fatalf("got %d expected login CSRF rejection", w.Code)
			}
		})
	}
}

type invalidMailProof struct{}

func (invalidMailProof) Verify(context.Context, string) (ports.FirebaseIdentity, error) {
	return ports.FirebaseIdentity{}, errors.New("invalid")
}
func (invalidMailProof) ValidSession(context.Context, string, time.Time, string) (bool, error) {
	return false, nil
}
func (invalidMailProof) DeleteIdentity(context.Context, string) error { return nil }
func (invalidMailProof) SendPush(context.Context, string) error       { return nil }

func TestFirebaseVerificationRequiresCSRFAndRejectsClientRecipient(t *testing.T) {
	h := &Handler{firebase: applicationauth.NewFirebaseService(invalidMailProof{}, nil, nil, nil), recovery: &applicationauth.RecoveryService{}}
	for _, tc := range []struct {
		name, body, csrf string
		status           int
	}{
		{"missing csrf", `{"id_token":"untrusted"}`, "", 403},
		{"client email", `{"id_token":"untrusted","email":"other@example.invalid"}`, strings.Repeat("a", 43), 400},
		{"forged proof", `{"id_token":"untrusted"}`, strings.Repeat("a", 43), 401},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "http://localhost:8080/api/v1/auth/firebase/verification", strings.NewReader(tc.body))
			r.AddCookie(&http.Cookie{Name: "wecratfs_csrf", Value: strings.Repeat("a", 43)})
			r.Header.Set("Origin", "http://localhost:8080")
			r.Header.Set("X-CSRF-Token", tc.csrf)
			w := httptest.NewRecorder()
			h.FirebaseVerificationRequest(w, r)
			if w.Code != tc.status || w.Header().Get("Cache-Control") != "private, no-store" {
				t.Fatalf("unsafe verification response: %d", w.Code)
			}
		})
	}
}

func TestPasswordRecoveryDoesNotRevealDeliveryOutage(t *testing.T) {
	h := &Handler{recovery: applicationauth.NewRecoveryService(nil, nil, nil, "https://weevcrafts.com")}
	for _, email := range []string{"customer@example.invalid", "unknown@example.invalid"} {
		r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/password/reset/request", strings.NewReader(`{"email":"`+email+`"}`))
		w := httptest.NewRecorder()
		h.PasswordResetRequest(w, r)
		if w.Code != http.StatusAccepted || !strings.Contains(w.Body.String(), "accepted") {
			t.Fatal("public delivery failure reveals account existence")
		}
	}
}
