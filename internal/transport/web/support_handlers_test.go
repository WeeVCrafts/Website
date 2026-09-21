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
	applicationcommerce "github.com/wecratfs/commerce/internal/application/commerce"
	domaincommerce "github.com/wecratfs/commerce/internal/domain/commerce"
	domainidentity "github.com/wecratfs/commerce/internal/domain/identity"
	"github.com/wecratfs/commerce/internal/ports"
)

type supportReplyCapture struct {
	called   bool
	actorID  int64
	ticketID int64
	message  string
}

func (r *supportReplyCapture) ReplySupportTicket(_ context.Context, actorID, ticketID int64, message string) error {
	r.called, r.actorID, r.ticketID, r.message = true, actorID, ticketID, message
	return nil
}

func TestSupportReplyRouteRequiresCSRFAndSupportAgentRole(t *testing.T) {
	session := ports.SessionRecord{ID: "support-reply-session", UserID: 42, CSRFToken: "support-reply-csrf", ExpiresAt: time.Now().Add(time.Hour)}
	capture := &supportReplyCapture{}
	makeHandler := func(role domainidentity.Role) *Handler {
		sessions := applicationauth.NewSessionService(&webSessionStore{session: session})
		handler, err := NewFullHandler(applicationcatalog.NewService(repository{}), nil, sessions, nil, nil, nil, nil, nil, false)
		if err != nil {
			t.Fatalf("create support reply handler: %v", err)
		}
		roleChecker := operationRoles{roles: map[int64]map[domainidentity.Role]bool{42: {role: true}}}
		serviceRepository := supportReplyServiceRepository{supportReplyCapture: capture}
		handler.SetSupportService(applicationcommerce.NewSupportService(serviceRepository, roleChecker))
		return handler
	}

	post := func(handler *Handler, csrf string) *httptest.ResponseRecorder {
		values := url.Values{"csrf_token": {csrf}, "ticket_id": {"91"}, "message": {"We have arranged a replacement."}}
		request := httptest.NewRequest(http.MethodPost, "/support/reply", strings.NewReader(values.Encode()))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		request.AddCookie(&http.Cookie{Name: "wecratfs_session", Value: session.ID})
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		return recorder
	}

	response := post(makeHandler(domainidentity.RoleSupportAgent), "wrong-token")
	if response.Code != http.StatusForbidden || capture.called {
		t.Fatalf("reply without CSRF should be denied, code=%d called=%t", response.Code, capture.called)
	}

	response = post(makeHandler(domainidentity.RoleCustomer), session.CSRFToken)
	if response.Code != http.StatusForbidden || capture.called {
		t.Fatalf("customer reply should be denied, code=%d called=%t", response.Code, capture.called)
	}

	response = post(makeHandler(domainidentity.RoleSupportAgent), session.CSRFToken)
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/support/cases?ticket_id=91" {
		t.Fatalf("support agent reply should return to its case, code=%d location=%q", response.Code, response.Header().Get("Location"))
	}
	if !capture.called || capture.actorID != session.UserID || capture.ticketID != 91 || capture.message != "We have arranged a replacement." {
		t.Fatalf("support reply was not persisted with the expected content: %+v", capture)
	}
}

type supportReplyServiceRepository struct {
	*supportReplyCapture
}

func (supportReplyServiceRepository) CreateSupportTicket(context.Context, int64, string, string, string) (domaincommerce.SupportTicket, error) {
	return domaincommerce.SupportTicket{}, nil
}
func (supportReplyServiceRepository) ListCustomerSupportTickets(context.Context, int64) ([]domaincommerce.SupportTicket, error) {
	return nil, nil
}
func (supportReplyServiceRepository) ListSellerSupportTickets(context.Context, int64) ([]domaincommerce.SupportTicket, error) {
	return nil, nil
}
func (supportReplyServiceRepository) ListSupportTickets(context.Context, int) ([]domaincommerce.SupportTicket, error) {
	return nil, nil
}
func (supportReplyServiceRepository) UpdateSupportTicket(context.Context, int64, int64, string, string) error {
	return nil
}
