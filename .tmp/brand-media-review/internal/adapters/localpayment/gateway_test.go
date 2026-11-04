package localpayment

import (
	"context"
	"testing"

	"github.com/wecratfs/commerce/internal/ports"
)

func TestGatewayCreatesAndVerifiesDeterministicOrder(t *testing.T) {
	gateway := NewGateway("test-secret")
	order, err := gateway.CreateOrder(context.Background(), ports.PaymentOrderRequest{Reference: "WC-DEMO-1", AmountCents: 1250, Currency: "INR"})
	if err != nil {
		t.Fatalf("create order: %v", err)
	}
	if order.ProviderOrderID != "local_order_WC-DEMO-1" {
		t.Fatalf("unexpected provider order id: %q", order.ProviderOrderID)
	}
	if err := gateway.VerifyPayment(context.Background(), ports.PaymentVerification{
		ProviderOrderID:   order.ProviderOrderID,
		ProviderPaymentID: "local_payment_WC-DEMO-1",
		Signature:         gateway.Sign(order.ProviderOrderID + "|local_payment_WC-DEMO-1"),
	}); err != nil {
		t.Fatalf("verify payment: %v", err)
	}
}

func TestGatewayVerifiesWebhookAndRefundsLocally(t *testing.T) {
	gateway := NewGateway("test-secret")
	body, err := WebhookPayload("evt-1", "payment.captured", "local_order_WC-DEMO-1", "local_payment_WC-DEMO-1", "captured", 1250, "INR")
	if err != nil {
		t.Fatalf("payload: %v", err)
	}
	webhook, err := gateway.VerifyWebhook(context.Background(), body, gateway.Sign(string(body)))
	if err != nil {
		t.Fatalf("verify webhook: %v", err)
	}
	if webhook.EventType != "payment.captured" || webhook.AmountCents != 1250 {
		t.Fatalf("unexpected webhook: %+v", webhook)
	}
	refund, err := gateway.CreateRefund(context.Background(), ports.PaymentRefundRequest{ProviderPaymentID: "local_payment_WC-DEMO-1", AmountCents: 1250, Currency: "INR", Receipt: "refund-1"})
	if err != nil || refund.Status != "processed" {
		t.Fatalf("refund failed: %+v %v", refund, err)
	}
}
