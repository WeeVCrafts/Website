package payment

import (
	"context"
	"errors"
	"github.com/wecratfs/commerce/internal/ports"
	"testing"
)

type recoveryOrderGateway struct {
	fakePaymentGateway
	events  []ports.PaymentWebhook
	lookups int
}

func (g *recoveryOrderGateway) LookupPayments(context.Context, ports.PaymentOrderRecord) ([]ports.PaymentWebhook, error) {
	g.lookups++
	return g.events, nil
}

type recoveryOrderRepository struct {
	fakePaymentOrderRepository
	writes    int
	claim     bool
	forbidden bool
}

func (r *recoveryOrderRepository) GetPaymentOrder(ctx context.Context, user int64, number string) (ports.PaymentOrderRecord, error) {
	if r.forbidden {
		return ports.PaymentOrderRecord{}, ports.ErrPaymentNotFound
	}
	return r.record, nil
}
func (r *recoveryOrderRepository) ClaimPaymentReconciliation(context.Context, int) ([]ports.PaymentOrderRecord, error) {
	return []ports.PaymentOrderRecord{r.record}, nil
}
func (r *recoveryOrderRepository) ClaimOrderPaymentReconciliation(context.Context, int64, string) (bool, error) {
	return r.claim, nil
}
func (r *recoveryOrderRepository) ProcessWebhook(context.Context, ports.PaymentWebhook, []byte) (bool, error) {
	r.writes++
	return true, nil
}
func TestPaymentReconciliationRecoversMissingWebhookWithoutCreatingPayment(t *testing.T) {
	repo := &recoveryOrderRepository{fakePaymentOrderRepository: fakePaymentOrderRepository{record: ports.PaymentOrderRecord{ProviderOrderID: "order_qa", AmountCents: 1000, Currency: "INR"}}, claim: true}
	gateway := &recoveryOrderGateway{events: []ports.PaymentWebhook{{EventID: "lookup_1", EventType: "payment.captured", ProviderOrderID: "order_qa", ProviderPaymentID: "pay_qa", AmountCents: 1000, Currency: "INR"}}}
	service := NewOrderService(gateway, repo)
	if n, err := service.Reconcile(context.Background(), 20); err != nil || n != 1 || repo.writes != 1 || gateway.created != 0 {
		t.Fatalf("recovery n=%d err=%v writes=%d", n, err, repo.writes)
	}
	repo.writes = 0
	gateway.events = append(gateway.events, ports.PaymentWebhook{ProviderOrderID: "different"})
	if _, err := service.Reconcile(context.Background(), 20); !errors.Is(err, ports.ErrPaymentMismatch) || repo.writes != 0 {
		t.Fatalf("partial mismatched recovery: %v writes=%d", err, repo.writes)
	}
	repo.forbidden = true
	before := gateway.lookups
	if err := service.Refresh(context.Background(), 999, "other-order"); !errors.Is(err, ports.ErrPaymentNotFound) || gateway.lookups != before {
		t.Fatalf("ownership bypass: %v", err)
	}
}
