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

type supportCreateCapture struct {
	called      bool
	userID      int64
	orderNumber string
	subject     string
	message     string
}

func (r *supportCreateCapture) CreateSupportTicket(_ context.Context, userID int64, orderNumber, subject, message string) (domaincommerce.SupportTicket, error) {
	r.called, r.userID = true, userID
	r.orderNumber, r.subject, r.message = orderNumber, subject, message
	return domaincommerce.SupportTicket{ID: 123, TicketNumber: "SUP-123", Subject: subject, Message: message, Status: "open"}, nil
}

type supportCreateServiceRepository struct {
	*supportCreateCapture
}

func (supportCreateServiceRepository) ListCustomerSupportTickets(context.Context, int64) ([]domaincommerce.SupportTicket, error) {
	return nil, nil
}
func (supportCreateServiceRepository) ListSellerSupportTickets(context.Context, int64) ([]domaincommerce.SupportTicket, error) {
	return nil, nil
}
func (supportCreateServiceRepository) ListSupportTickets(context.Context, int) ([]domaincommerce.SupportTicket, error) {
	return nil, nil
}
func (supportCreateServiceRepository) UpdateSupportTicket(context.Context, int64, int64, string, string) error {
	return nil
}
func (supportCreateServiceRepository) ReplySupportTicket(context.Context, int64, int64, string) error {
	return nil
}

func TestSupportCreateRoutePersistsCustomerRequest(t *testing.T) {
	session := ports.SessionRecord{ID: "support-create-session", UserID: 42, CSRFToken: "support-create-csrf", ExpiresAt: time.Now().Add(time.Hour)}
	capture := &supportCreateCapture{}
	sessions := applicationauth.NewSessionService(&webSessionStore{session: session})
	handler, err := NewFullHandler(applicationcatalog.NewService(repository{}), nil, sessions, nil, nil, nil, nil, nil, false)
	if err != nil {
		t.Fatalf("create support handler: %v", err)
	}
	roles := operationRoles{roles: map[int64]map[domainidentity.Role]bool{42: {domainidentity.RoleCustomer: true}}}
	handler.SetSupportService(applicationcommerce.NewSupportService(supportCreateServiceRepository{supportCreateCapture: capture}, roles))

	values := url.Values{
		"csrf_token":   {session.CSRFToken},
		"order_number": {"WC-42"},
		"subject":      {"Delivery question"},
		"message":      {"Please confirm the delivery window for this order."},
	}
	request := httptest.NewRequest(http.MethodPost, "/support/create", strings.NewReader(values.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.AddCookie(&http.Cookie{Name: "wecratfs_session", Value: session.ID})
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusSeeOther || recorder.Header().Get("Location") != "/account/support?notice=Support+request+created" {
		t.Fatalf("support create should redirect after persistence, code=%d location=%q", recorder.Code, recorder.Header().Get("Location"))
	}
	if !capture.called || capture.userID != session.UserID || capture.orderNumber != "WC-42" || capture.subject != "Delivery question" {
		t.Fatalf("support request was not persisted with expected fields: %+v", capture)
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
