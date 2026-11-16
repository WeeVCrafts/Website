package postgres

import (
	"context"
	"errors"
	"fmt"
	"testing"

	domaincommerce "github.com/wecratfs/commerce/internal/domain/commerce"
	"github.com/wecratfs/commerce/internal/ports"
)

func TestPrepaidFulfillmentRequiresCaptureAndEverySellerDelivery(t *testing.T) {
	pool := openIntegrationPool(t)
	second := newCommerceFixture(t, pool, 0, 1)
	first := newCommerceFixture(t, pool, 1, 1)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `INSERT INTO cart_items (cart_id, variant_id, quantity) VALUES ($1,$2,1)`, first.cartIDs[0], second.variantID); err != nil {
		t.Fatal(err)
	}
	repo, payments := NewCommerceRepository(pool), NewPaymentRepository(pool)
	customer := first.userIDs[0]
	order, err := repo.CreateOrder(ctx, customer, first.cartIDs[0], fmt.Sprintf("prepaid-fulfillment-%d", customer), domaincommerce.AddressInput{RecipientName: "Test Customer", Phone: "9876543210", Line1: "1 Test Lane", City: "Pune", State: "MH", PostalCode: "411001", CountryCode: "IN"})
	if err != nil {
		t.Fatal(err)
	}
	// This suite exercises the preserved historical seller-operated path.
	if _, err = pool.Exec(ctx, `UPDATE orders SET workflow_version=1 WHERE id=$1`, order.ID); err != nil {
		t.Fatal(err)
	}
	providerOrder := fmt.Sprintf("order_prepaid_%d", order.ID)
	providerPayment := fmt.Sprintf("pay_prepaid_%d", order.ID)
	if _, err = payments.AttachProviderOrder(ctx, customer, order.OrderNumber, ports.PaymentOrder{ProviderOrderID: providerOrder, AmountCents: order.TotalCents, Currency: order.Currency}); err != nil {
		t.Fatal(err)
	}
	owners := []int64{first.ownerID, second.ownerID}
	sellers := make([]int64, len(owners))
	for i, owner := range owners {
		if err := pool.QueryRow(ctx, `SELECT id FROM sellers WHERE owner_user_id=$1`, owner).Scan(&sellers[i]); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM payment_review_cases WHERE order_id=$1`, order.ID)
		_, _ = pool.Exec(ctx, `DELETE FROM payment_webhook_events WHERE event_id LIKE $1`, providerOrder+"%")
	})
	check := func(eligible, payable bool, paidEvents int) {
		t.Helper()
		for i, owner := range owners {
			orders, err := repo.ListSellerOrders(ctx, owner)
			if err != nil || (len(orders) == 1 && orders[0].ID == order.ID) != eligible {
				t.Fatalf("seller %d queue: eligible=%v, orders=%v, err=%v", i, eligible, orders, err)
			}
			if !eligible && !errors.Is(repo.UpdateSellerFulfillment(ctx, owner, sellers[i], order.OrderNumber, "processing", "", "", ""), ports.ErrInvalidState) {
				t.Fatalf("seller %d could start fulfillment without a cleared capture", i)
			}
		}
		gotPayable, err := payments.PaymentOrderPayable(ctx, customer, order.OrderNumber)
		if err != nil || gotPayable != payable {
			t.Fatalf("payable=%v, want %v, err=%v", gotPayable, payable, err)
		}
		var gotEvents int
		if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM outbox_events WHERE event_type='OrderPaid' AND aggregate_id=$1`, order.OrderNumber).Scan(&gotEvents); err != nil || gotEvents != paidEvents {
			t.Fatalf("OrderPaid events=%d, want %d, err=%v", gotEvents, paidEvents, err)
		}
	}
	event := func(kind, paymentID, suffix string) {
		t.Helper()
		_, err := payments.ProcessWebhook(ctx, ports.PaymentWebhook{EventID: providerOrder + suffix, EventType: kind, ProviderOrderID: providerOrder, ProviderPaymentID: paymentID, AmountCents: order.TotalCents, Currency: order.Currency}, []byte(`{}`))
		if err != nil {
			t.Fatal(err)
		}
	}
	check(false, true, 0)
	event("payment.pending", providerPayment+"_failed", "-pending")
	check(false, false, 0)
	event("payment.failed", providerPayment+"_failed", "-failed")
	check(false, true, 0)
	event("payment.authorized", providerPayment, "-authorized")
	check(false, false, 0)
	event("payment.captured", providerPayment, "-captured")
	check(true, false, 1)
	if err := repo.UpdateSellerFulfillment(ctx, owners[0], sellers[1], order.OrderNumber, "processing", "", "", ""); !errors.Is(err, ports.ErrOrderNotFound) {
		t.Fatalf("seller could update the other seller's shipment: %v", err)
	}
	for i, owner := range owners {
		for _, stage := range []string{"processing", "packing", "shipped", "out_for_delivery", "delivered"} {
			if err := repo.UpdateSellerFulfillment(ctx, owner, sellers[i], order.OrderNumber, stage, "Test Carrier", fmt.Sprintf("TEST%d", i), ""); err != nil {
				t.Fatalf("seller %d advancing to %s: %v", i, stage, err)
			}
		}
		detail, err := repo.GetOrder(ctx, customer, order.OrderNumber)
		want := []string{"processing", "delivered"}[i]
		if err != nil || detail.Status != want || len(detail.Fulfillments) != 2 {
			t.Fatalf("after seller %d: order status=%s want %s, deliveries=%d, err=%v", i, detail.Status, want, len(detail.Fulfillments), err)
		}
	}
	event("payment.captured", providerPayment, "-repeat")
	check(true, false, 1)
	event("payment.captured", providerPayment+"_extra", "-extra")
	check(false, false, 1)
}
