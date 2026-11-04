package payment

import (
	"context"
	"errors"
	"testing"

	"github.com/wecratfs/commerce/internal/ports"
)

type checkoutVerificationGateway struct {
	verifyErr error
	verified  ports.PaymentVerification
}

func (g *checkoutVerificationGateway) CreateOrder(context.Context, ports.PaymentOrderRequest) (ports.PaymentOrder, error) {
	return ports.PaymentOrder{}, errors.New("not used")
}

func (g *checkoutVerificationGateway) VerifyPayment(_ context.Context, verification ports.PaymentVerification) error {
	g.verified = verification
	return g.verifyErr
}

type checkoutVerificationRepository struct {
	record  ports.PaymentOrderRecord
	webhook ports.PaymentWebhook
	payload []byte
}

func (r *checkoutVerificationRepository) GetPaymentOrder(context.Context, int64, string) (ports.PaymentOrderRecord, error) {
	return r.record, nil
}

func (r *checkoutVerificationRepository) AttachProviderOrder(context.Context, int64, string, ports.PaymentOrder) (ports.PaymentOrderRecord, error) {
	return ports.PaymentOrderRecord{}, errors.New("not used")
}

func (r *checkoutVerificationRepository) ProcessWebhook(_ context.Context, webhook ports.PaymentWebhook, payload []byte) (bool, error) {
	r.webhook = webhook
	r.payload = append([]byte(nil), payload...)
	return true, nil
}

func TestCheckoutVerificationRecordsAuthorizedPaymentAfterProviderVerification(t *testing.T) {
	repository := &checkoutVerificationRepository{record: ports.PaymentOrderRecord{
		OrderID: 42, OrderNumber: "WC-42", OrderStatus: "pending_payment", Provider: "razorpay",
		ProviderOrderID: "order_42", PaymentStatus: "pending", AmountCents: 13900, Currency: "INR",
	}}
	gateway := &checkoutVerificationGateway{}
	input := ports.PaymentVerification{ProviderOrderID: "order_42", ProviderPaymentID: "pay_42", Signature: "valid"}
	service := NewCheckoutVerificationService(gateway, repository, repository)
	if err := service.Verify(context.Background(), 7, "WC-42", input); err != nil {
		t.Fatalf("verify checkout payment: %v", err)
	}
	if gateway.verified != input {
		t.Fatalf("gateway verified %+v, want %+v", gateway.verified, input)
	}
	if repository.webhook.EventType != "payment.authorized" || repository.webhook.ProviderPaymentID != "pay_42" || repository.webhook.AmountCents != 13900 || repository.webhook.Currency != "INR" || len(repository.payload) == 0 {
		t.Fatalf("unexpected authorization record: %+v", repository.webhook)
	}
}

func TestCheckoutVerificationRejectsMismatchedOrUnsignedPayment(t *testing.T) {
	repository := &checkoutVerificationRepository{record: ports.PaymentOrderRecord{
		OrderNumber: "WC-42", OrderStatus: "pending_payment", Provider: "razorpay",
		ProviderOrderID: "order_42", PaymentStatus: "pending", AmountCents: 13900, Currency: "INR",
	}}
	gateway := &checkoutVerificationGateway{verifyErr: errors.New("bad signature")}
	service := NewCheckoutVerificationService(gateway, repository, repository)
	input := ports.PaymentVerification{ProviderOrderID: "order_42", ProviderPaymentID: "pay_42", Signature: "bad"}
	if err := service.Verify(context.Background(), 7, "WC-42", input); !errors.Is(err, ErrCheckoutPaymentInvalid) {
		t.Fatalf("invalid signature error = %v", err)
	}
	if len(repository.payload) != 0 {
		t.Fatal("invalid signature must not reach the payment repository")
	}
	input.ProviderOrderID = "another_order"
	if err := service.Verify(context.Background(), 7, "WC-42", input); !errors.Is(err, ErrCheckoutPaymentInvalid) {
		t.Fatalf("mismatched provider order error = %v", err)
	}
}
