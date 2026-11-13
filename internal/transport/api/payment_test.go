package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	applicationauth "github.com/wecratfs/commerce/internal/application/auth"
	applicationpayment "github.com/wecratfs/commerce/internal/application/payment"
	"github.com/wecratfs/commerce/internal/ports"
)

type paymentHTTPGateway struct {
	createErr error
	verifyErr error
	created   int
}

func (g *paymentHTTPGateway) CreateOrder(_ context.Context, request ports.PaymentOrderRequest) (ports.PaymentOrder, error) {
	g.created++
	if g.createErr != nil {
		return ports.PaymentOrder{}, g.createErr
	}
	return ports.PaymentOrder{ProviderOrderID: "order_http", AmountCents: request.AmountCents, Currency: request.Currency}, nil
}

func (g *paymentHTTPGateway) VerifyPayment(context.Context, ports.PaymentVerification) error {
	return g.verifyErr
}

type paymentHTTPRepository struct {
	record   ports.PaymentOrderRecord
	webhooks int
}

func (r *paymentHTTPRepository) GetPaymentOrder(context.Context, int64, string) (ports.PaymentOrderRecord, error) {
	return r.record, nil
}

func (r *paymentHTTPRepository) AttachProviderOrder(_ context.Context, _ int64, _ string, order ports.PaymentOrder) (ports.PaymentOrderRecord, error) {
	r.record.ProviderOrderID = order.ProviderOrderID
	r.record.PaymentStatus = "pending"
	return r.record, nil
}

func (r *paymentHTTPRepository) ProcessWebhook(context.Context, ports.PaymentWebhook, []byte) (bool, error) {
	r.webhooks++
	return true, nil
}

type paymentHTTPSessionStore struct {
	record ports.SessionRecord
}

func (s *paymentHTTPSessionStore) Create(context.Context, ports.SessionRecord) error { return nil }
func (s *paymentHTTPSessionStore) Get(context.Context, string) (ports.SessionRecord, error) {
	return s.record, nil
}
func (s *paymentHTTPSessionStore) Delete(context.Context, string) error   { return nil }
func (s *paymentHTTPSessionStore) DeleteAll(context.Context, int64) error { return nil }
func (s *paymentHTTPSessionStore) List(context.Context, int64) ([]ports.SessionRecord, error) {
	return []ports.SessionRecord{s.record}, nil
}

func newPaymentHTTPHandler(record ports.PaymentOrderRecord, gateway *paymentHTTPGateway) (*Handler, *paymentHTTPRepository, ports.SessionRecord) {
	repository := &paymentHTTPRepository{record: record}
	session := ports.SessionRecord{ID: "payment-http-session", UserID: 42, CSRFToken: "payment-http-csrf", ExpiresAt: time.Now().Add(time.Hour)}
	sessions := applicationauth.NewSessionService(&paymentHTTPSessionStore{record: session})
	handler := &Handler{
		sessions:        sessions,
		payment:         applicationpayment.NewOrderService(gateway, repository),
		checkoutPayment: applicationpayment.NewCheckoutVerificationService(gateway, repository, repository),
	}
	return handler, repository, session
}

func paymentRequest(method, path, csrf string, body string, session ports.SessionRecord) *http.Request {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.AddCookie(&http.Cookie{Name: "wecratfs_session", Value: session.ID})
	if csrf != "" {
		request.Header.Set("X-CSRF-Token", csrf)
	}
	request.Header.Set("Content-Type", "application/json")
	return request
}

func TestPaymentOrderHTTPMinimumAndAuthenticationContracts(t *testing.T) {
	base := ports.PaymentOrderRecord{OrderID: 42, OrderNumber: "WC-42", OrderStatus: "pending_payment", Provider: "razorpay", PaymentStatus: "created", Currency: "INR"}
	tests := []struct {
		name       string
		amount     int64
		csrf       string
		cookie     bool
		wantStatus int
	}{
		{name: "99 paise rejected before provider", amount: 99, csrf: "payment-http-csrf", cookie: true, wantStatus: http.StatusBadRequest},
		{name: "100 paise accepted", amount: 100, csrf: "payment-http-csrf", cookie: true, wantStatus: http.StatusCreated},
		{name: "missing authentication", amount: 100, csrf: "", cookie: false, wantStatus: http.StatusUnauthorized},
		{name: "failed csrf", amount: 100, csrf: "wrong", cookie: true, wantStatus: http.StatusForbidden},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			record := base
			record.AmountCents = test.amount
			gateway := &paymentHTTPGateway{}
			handler, _, session := newPaymentHTTPHandler(record, gateway)
			request := httptest.NewRequest(http.MethodPost, "/api/v1/orders/WC-42/payment", strings.NewReader(`{}`))
			if test.cookie {
				request.AddCookie(&http.Cookie{Name: "wecratfs_session", Value: session.ID})
			}
			request.Header.Set("X-CSRF-Token", test.csrf)
			recorder := httptest.NewRecorder()
			handler.PaymentOrder(recorder, request)
			if recorder.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d: %s", recorder.Code, test.wantStatus, recorder.Body.String())
			}
			if test.amount == 99 && gateway.created != 0 {
				t.Fatalf("minimum rejection called provider %d times", gateway.created)
			}
		})
	}
}

func TestPaymentOrderHTTPMapsProviderFailureToGenericServerError(t *testing.T) {
	record := ports.PaymentOrderRecord{OrderID: 42, OrderNumber: "WC-42", OrderStatus: "pending_payment", Provider: "razorpay", PaymentStatus: "created", AmountCents: 100, Currency: "INR"}
	gateway := &paymentHTTPGateway{createErr: errors.New("provider credentials rejected")}
	handler, _, session := newPaymentHTTPHandler(record, gateway)
	recorder := httptest.NewRecorder()
	handler.PaymentOrder(recorder, paymentRequest(http.MethodPost, "/api/v1/orders/WC-42/payment", session.CSRFToken, `{}`, session))
	if recorder.Code != http.StatusInternalServerError || strings.Contains(recorder.Body.String(), "credentials") {
		t.Fatalf("provider failure leaked or returned wrong status: %d %s", recorder.Code, recorder.Body.String())
	}
}

func TestVerifyCheckoutPaymentRejectsTamperedCallbacksWithoutMutation(t *testing.T) {
	record := ports.PaymentOrderRecord{OrderID: 42, OrderNumber: "WC-42", OrderStatus: "pending_payment", Provider: "razorpay", ProviderOrderID: "order_http", PaymentStatus: "pending", AmountCents: 100, Currency: "INR"}
	gateway := &paymentHTTPGateway{verifyErr: errors.New("bad signature")}
	handler, repository, session := newPaymentHTTPHandler(record, gateway)
	for _, body := range []string{
		`{"provider_order_id":"order_http","provider_payment_id":"pay_http","signature":""}`,
		`{"provider_order_id":"order_tampered","provider_payment_id":"pay_http","signature":"sig"}`,
		`{"provider_order_id":"order_http","provider_payment_id":"pay_http","signature":"sig"}`,
	} {
		recorder := httptest.NewRecorder()
		handler.VerifyCheckoutPayment(recorder, paymentRequest(http.MethodPost, "/api/v1/orders/WC-42/payment/verify", session.CSRFToken, body, session))
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("callback %s returned %d, want 400: %s", body, recorder.Code, recorder.Body.String())
		}
	}
	if repository.webhooks != 0 {
		t.Fatalf("rejected callbacks mutated payment state %d times", repository.webhooks)
	}
}
