package postgres

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	applicationpayment "github.com/wecratfs/commerce/internal/application/payment"
	domaincommerce "github.com/wecratfs/commerce/internal/domain/commerce"
	"github.com/wecratfs/commerce/internal/ports"
)

type integrationRefundGateway struct {
	calls   int
	status  string
	request ports.PaymentRefundRequest
}

func (g *integrationRefundGateway) CreateRefund(_ context.Context, request ports.PaymentRefundRequest) (ports.PaymentRefund, error) {
	g.calls++
	g.request = request
	status := g.status
	if status == "" {
		status = "processed"
	}
	return ports.PaymentRefund{
		ProviderRefundID:  "rfnd_fixture_" + fmt.Sprint(g.calls),
		ProviderPaymentID: request.ProviderPaymentID,
		AmountCents:       request.AmountCents,
		Currency:          request.Currency,
		Status:            status,
	}, nil
}

func (g *integrationRefundGateway) LookupRefund(context.Context, ports.PaymentRefundRecord) (ports.PaymentRefund, error) {
	return ports.PaymentRefund{ProviderRefundID: "rfnd_fixture_1", ProviderPaymentID: g.request.ProviderPaymentID, AmountCents: g.request.AmountCents, Currency: g.request.Currency, Status: "processed"}, nil
}

func TestRefundAfterReceivedReturnTransitionsDeliveredOrderAndReplays(t *testing.T) {
	pool := openIntegrationPool(t)
	fixture := newCommerceFixture(t, pool, 1, 1)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	address := domaincommerce.AddressInput{RecipientName: "Refund Fixture", Phone: "9876543210", Line1: "1 Test Lane", City: "Pune", State: "MH", PostalCode: "411001", CountryCode: "IN"}
	order, err := NewCommerceRepository(pool).CreateOrder(ctx, fixture.userIDs[0], fixture.cartIDs[0], "refund-integration-key-1234", address)
	if err != nil {
		t.Fatalf("create pending order: %v", err)
	}
	paymentRepository := NewPaymentRepository(pool)
	providerOrderID := "order_refund_fixture_" + fmt.Sprint(order.ID)
	providerPaymentID := "pay_refund_fixture_" + fmt.Sprint(order.ID)
	if _, err := paymentRepository.AttachProviderOrder(ctx, fixture.userIDs[0], order.OrderNumber, ports.PaymentOrder{ProviderOrderID: providerOrderID, AmountCents: order.TotalCents, Currency: order.Currency}); err != nil {
		t.Fatalf("attach provider order: %v", err)
	}
	eventID := "evt_refund_fixture_" + fmt.Sprint(order.ID)
	if processed, err := paymentRepository.ProcessWebhook(ctx, ports.PaymentWebhook{EventID: eventID, EventType: "payment.captured", ProviderOrderID: providerOrderID, ProviderPaymentID: providerPaymentID, AmountCents: order.TotalCents, Currency: order.Currency}, []byte(`{"event":"payment.captured"}`)); err != nil || !processed {
		t.Fatalf("capture payment: processed=%v err=%v", processed, err)
	}
	t.Cleanup(func() {
		cleanupContext, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_, _ = pool.Exec(cleanupContext, `DELETE FROM payment_refunds WHERE order_id = $1`, order.ID)
		_, _ = pool.Exec(cleanupContext, `DELETE FROM audit_logs WHERE resource_id = $1`, fmt.Sprint(order.ID))
		_, _ = pool.Exec(cleanupContext, `DELETE FROM payment_webhook_events WHERE event_id = $1`, eventID)
		_, _ = pool.Exec(cleanupContext, `DELETE FROM payment_webhook_events WHERE event_id = $1`, eventID+"_refund")
	})

	gateway := &integrationRefundGateway{status: "pending"}
	service := applicationpayment.NewRefundService(gateway, paymentRepository)
	if _, err := service.Refund(ctx, fixture.userIDs[0], order.OrderNumber, "refund-request-key-1234", "customer request"); !errors.Is(err, ports.ErrRefundState) {
		t.Fatalf("refund should be blocked until operations receive the returned item, got %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE orders SET status='delivered' WHERE id=$1`, order.ID); err != nil {
		t.Fatalf("mark order delivered: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO return_requests(order_id,user_id,reason,status) VALUES($1,$2,'fixture return','received')`, order.ID, fixture.userIDs[0]); err != nil {
		t.Fatalf("record received return: %v", err)
	}
	refund, err := service.Refund(ctx, fixture.userIDs[0], order.OrderNumber, "refund-request-key-1234", "customer request")
	if err != nil {
		t.Fatalf("process refund: %v", err)
	}
	if refund.Status != "pending" || refund.AmountCents != order.TotalCents || gateway.calls != 1 {
		t.Fatalf("unexpected refund result: refund=%+v gateway_calls=%d", refund, gateway.calls)
	}
	if claimed, err := paymentRepository.ClaimRefundSubmission(ctx, refund.ID); err != nil || claimed {
		t.Fatalf("submitted refund claimed again: %v %v", claimed, err)
	}
	if _, err := pool.Exec(ctx, `UPDATE payment_refunds SET next_reconcile_at=NOW() WHERE id=$1`, refund.ID); err != nil {
		t.Fatal(err)
	}
	if count, err := service.Reconcile(ctx, 20); err != nil || count != 1 {
		t.Fatalf("reconcile: %d %v", count, err)
	}
	if count, err := service.Reconcile(ctx, 20); err != nil || count != 0 {
		t.Fatalf("reconcile repeated: %d %v", count, err)
	}
	refundEvent := ports.PaymentWebhook{EventID: eventID + "_refund", EventType: "refund.processed", ProviderRefundID: "rfnd_fixture_1", ProviderPaymentID: providerPaymentID, AmountCents: order.TotalCents, Currency: order.Currency, Status: "processed"}
	if processed, err := paymentRepository.ProcessWebhook(ctx, refundEvent, []byte(`{"event":"refund.processed"}`)); err != nil || !processed {
		t.Fatalf("refund webhook: %v %v", processed, err)
	}
	if processed, err := paymentRepository.ProcessWebhook(ctx, refundEvent, []byte(`{"event":"refund.processed"}`)); err != nil || processed {
		t.Fatalf("refund webhook replay: %v %v", processed, err)
	}

	var orderStatus, paymentStatus, refundStatus string
	if err := pool.QueryRow(ctx, `SELECT o.status, pa.status, pr.status FROM orders o JOIN payment_attempts pa ON pa.order_id = o.id JOIN payment_refunds pr ON pr.order_id = o.id WHERE o.id = $1`, order.ID).Scan(&orderStatus, &paymentStatus, &refundStatus); err != nil {
		t.Fatalf("read refund state: %v", err)
	}
	if orderStatus != "refunded" || paymentStatus != "refunded" || refundStatus != "processed" {
		t.Fatalf("unexpected refund state: order=%s payment=%s refund=%s", orderStatus, paymentStatus, refundStatus)
	}
	var historyCount, auditCount int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM order_status_history WHERE order_id = $1 AND to_status = 'refunded'`, order.ID).Scan(&historyCount); err != nil {
		t.Fatalf("read refund history: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM audit_logs WHERE action = 'payment.refund_processed' AND resource_id = $1`, fmt.Sprint(order.ID)).Scan(&auditCount); err != nil {
		t.Fatalf("read refund audit: %v", err)
	}
	if historyCount != 1 || auditCount != 1 {
		t.Fatalf("refund transition was not audited once: history=%d audit=%d", historyCount, auditCount)
	}

	replayed, err := service.Refund(ctx, fixture.userIDs[0], order.OrderNumber, "refund-request-key-1234", "customer request")
	if err != nil || replayed.ID != refund.ID || gateway.calls != 1 {
		t.Fatalf("refund replay was not idempotent: refund=%+v err=%v gateway_calls=%d", replayed, err, gateway.calls)
	}
	if _, err := service.Refund(ctx, fixture.userIDs[0], order.OrderNumber, "another-refund-key-1234", "duplicate"); !errors.Is(err, ports.ErrRefundState) {
		t.Fatalf("expected a different key to be rejected, got %v", err)
	}
}
