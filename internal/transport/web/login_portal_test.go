package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	applicationauth "github.com/wecratfs/commerce/internal/application/auth"
	applicationcatalog "github.com/wecratfs/commerce/internal/application/catalog"
	domainidentity "github.com/wecratfs/commerce/internal/domain/identity"
	"github.com/wecratfs/commerce/internal/ports"
)

type portalLoginRepository struct {
	user domainidentity.User
}

func (r portalLoginRepository) CreateCustomer(context.Context, string, string, string, string) (domainidentity.User, error) {
	return domainidentity.User{}, nil
}
func (r portalLoginRepository) GetCredentialsByEmail(context.Context, string) (domainidentity.Credentials, error) {
	return domainidentity.Credentials{User: r.user, PasswordHash: "valid"}, nil
}
func (portalLoginRepository) RecordFailedLogin(context.Context, int64) error { return nil }
func (portalLoginRepository) ResetFailedLogin(context.Context, int64) error  { return nil }

type portalLoginHasher struct{}

func (portalLoginHasher) Hash(string) (string, error) { return "valid", nil }
func (portalLoginHasher) Compare(encoded, password string) bool {
	return encoded == "valid" && password == "password"
}

func TestWebLoginRedirectsIntoRoleWorkspace(t *testing.T) {
	tests := []struct {
		name  string
		roles []domainidentity.Role
		want  string
	}{
		{name: "customer", roles: []domainidentity.Role{domainidentity.RoleCustomer}, want: "/products"},
		{name: "seller", roles: []domainidentity.Role{domainidentity.RoleSeller}, want: "/seller-admin"},
		{name: "admin", roles: []domainidentity.Role{domainidentity.RoleAdmin}, want: "/admin"},
		{name: "super admin", roles: []domainidentity.Role{domainidentity.RoleSuperAdmin}, want: "/super-admin"},
		{name: "multiple roles", roles: []domainidentity.Role{domainidentity.RoleCustomer, domainidentity.RoleAdmin}, want: "/access-unavailable"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			user := domainidentity.User{ID: 19, Email: "qa@example.test", DisplayName: "QA Account", Status: "active", Roles: test.roles}
			authService := applicationauth.NewService(portalLoginRepository{user: user}, portalLoginHasher{})
			sessionStore := &webSessionStore{}
			sessionService := applicationauth.NewSessionService(sessionStore)
			handler, err := NewHandlerWithAuth(applicationcatalog.NewService(repository{}), authService, sessionService, false)
			if err != nil {
				t.Fatalf("create handler: %v", err)
			}

			form := url.Values{"email": {user.Email}, "password": {"password"}, "csrf_token": {"portal-csrf"}}
			request := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
			request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			request.AddCookie(&http.Cookie{Name: "wecratfs_csrf", Value: "portal-csrf"})
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)

			if recorder.Code != http.StatusSeeOther || recorder.Header().Get("Location") != test.want {
				t.Fatalf("login redirect = %d %q, want 303 %q: %s", recorder.Code, recorder.Header().Get("Location"), test.want, recorder.Body.String())
			}
			if sessionStore.session.UserID != user.ID || sessionStore.session.ExpiresAt.Before(time.Now()) {
				t.Fatalf("login session was not established for user %d: %+v", user.ID, sessionStore.session)
			}
		})
	}
}

var _ ports.UserRepository = portalLoginRepository{}
