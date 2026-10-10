package main

import (
	"context"
	applicationauth "github.com/wecratfs/commerce/internal/application/auth"
	"github.com/wecratfs/commerce/internal/ports"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestFirebaseProductionRejectsEmulators(t *testing.T) {
	t.Setenv("FIREBASE_AUTH_EMULATOR_HOST", "localhost:9099")
	if validateFirebaseEnvironment("production") == nil {
		t.Fatal("production accepted unsigned emulator tokens")
	}
}
func TestFirebaseConfigurationDoesNotExposeServerSecrets(t *testing.T) {
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "private-credential-fixture")
	t.Setenv("WECRATFS_FIREBASE_PROJECT_ID", "public-project")
	r := httptest.NewRequest(http.MethodGet, "/api/v1/firebase/config", nil)
	w := httptest.NewRecorder()
	firebaseConfiguration(true)(w, r)
	if strings.Contains(w.Body.String(), "private-credential-fixture") || strings.Contains(w.Body.String(), "GOOGLE_APPLICATION_CREDENTIALS") {
		t.Fatal("server credential leaked")
	}
	if !strings.Contains(w.Body.String(), "public-project") {
		t.Fatal("public browser config missing")
	}
}

type pushSessionFixture struct{ record ports.SessionRecord }

func (s pushSessionFixture) Create(context.Context, ports.SessionRecord) error { return nil }
func (s pushSessionFixture) Get(context.Context, string) (ports.SessionRecord, error) {
	return s.record, nil
}
func (s pushSessionFixture) Delete(context.Context, string) error   { return nil }
func (s pushSessionFixture) DeleteAll(context.Context, int64) error { return nil }
func (s pushSessionFixture) List(context.Context, int64) ([]ports.SessionRecord, error) {
	return nil, nil
}
func TestPushRejectsUnauthenticatedRequestsBeforeDatabase(t *testing.T) {
	p := &customerPush{}
	w := httptest.NewRecorder()
	p.Handle(w, httptest.NewRequest(http.MethodGet, "/api/v1/customer/push", nil))
	if w.Code != 401 {
		t.Fatalf("got %d", w.Code)
	}
}
func TestFirebaseWorkerServedAtRootWithoutPrivateCache(t *testing.T) {
	w := httptest.NewRecorder()
	firebaseWorker(w, httptest.NewRequest(http.MethodGet, "/firebase-messaging-sw.js", nil))
	if w.Code != 200 || w.Header().Get("Service-Worker-Allowed") != "/" || !strings.Contains(w.Body.String(), "/account/notifications") {
		t.Fatal("worker registration contract failed")
	}
	if strings.Contains(w.Body.String(), "caches.open") {
		t.Fatal("worker should not cache customer pages")
	}
}

var _ ports.SessionStore = pushSessionFixture{}

func TestPushEnforcesCustomerRoleAndCSRF(t *testing.T) {
	for _, tc := range []struct {
		name  string
		roles []string
		csrf  string
	}{{"seller", []string{"seller_owner"}, "csrf"}, {"ambiguous", []string{"customer", "seller_owner"}, "csrf"}, {"missing csrf", []string{"customer"}, ""}} {
		t.Run(tc.name, func(t *testing.T) {
			store := pushSessionFixture{ports.SessionRecord{ID: "fixture", UserID: 10, RoleSlugs: tc.roles, CSRFToken: "csrf", ExpiresAt: time.Now().Add(time.Hour)}}
			p := &customerPush{sessions: applicationauth.NewSessionService(store)}
			r := httptest.NewRequest(http.MethodPost, "/api/v1/customer/push", strings.NewReader(`{"token":"fixture"}`))
			r.AddCookie(&http.Cookie{Name: "wecratfs_session", Value: "fixture"})
			r.Header.Set("X-CSRF-Token", tc.csrf)
			w := httptest.NewRecorder()
			p.Handle(w, r)
			if w.Code != 403 {
				t.Fatalf("got %d wanted denial before database", w.Code)
			}
		})
	}
}
