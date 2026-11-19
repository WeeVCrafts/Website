package postgres

import (
	"context"
	"fmt"
	domain "github.com/wecratfs/commerce/internal/domain/commerce"
	"github.com/wecratfs/commerce/internal/ports"
	"testing"
)

func mixedCommerceFixture(t *testing.T, x marketplaceFixture) (marketplaceFixture, marketplaceFixture) {
	t.Helper()
	ctx := context.Background()
	other := newMarketplaceFixtureWithPool(t, x.pool)
	if _, err := x.pool.Exec(ctx, `UPDATE products SET name='Second seller private product' WHERE id=$1`, other.f.productID); err != nil {
		t.Fatal(err)
	}
	if _, err := x.pool.Exec(ctx, `UPDATE product_variants SET return_class='fragile_art' WHERE id=ANY($1)`, []int64{x.f.variantID, other.f.variantID}); err != nil {
		t.Fatal(err)
	}
	var cart int64
	if err := x.pool.QueryRow(ctx, `INSERT INTO carts(user_id,expires_at) VALUES($1,now()+interval '1 hour') RETURNING id`, x.f.userIDs[0]).Scan(&cart); err != nil {
		t.Fatal(err)
	}
	if _, err := x.pool.Exec(ctx, `INSERT INTO cart_items(cart_id,variant_id,quantity) VALUES($1,$2,3),($1,$3,2)`, cart, x.f.variantID, other.f.variantID); err != nil {
		t.Fatal(err)
	}
	mixed := x
	var err error
	mixed.order, err = x.commerce.CreateOrder(ctx, x.f.userIDs[0], cart, fmt.Sprintf("mixed-basket-%d", cart), domain.AddressInput{RecipientName: "Isolated buyer", Phone: "9000000000", Line1: "Test lane", City: "Pune", State: "MH", PostalCode: "411001", CountryCode: "IN"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = x.pool.Exec(ctx, `INSERT INTO order_delivery_decisions(order_id,seller_id,decision,estimate) VALUES($1,$2,'manual_fallback','{}'),($1,$3,'manual_fallback','{}')`, mixed.order.ID, x.seller, other.seller); err != nil {
		t.Fatal(err)
	}
	mixed.capture(t)
	c := mixed.command("cancel")
	c.Key = fmt.Sprintf("mixed-cancel-%016d", mixed.order.ID)
	c.Quantity = 1
	c.Reason = "Isolated partial cancellation before dispatch"
	if err = x.pool.QueryRow(ctx, `SELECT id FROM order_items WHERE order_id=$1 AND seller_id=$2`, mixed.order.ID, x.seller).Scan(&c.ItemID); err != nil {
		t.Fatal(err)
	}
	if err = x.repo.Act(ctx, x.f.userIDs[0], c); err != nil {
		t.Fatal(err)
	}
	var cancellation int64
	if err = x.pool.QueryRow(ctx, `SELECT id FROM marketplace_cancellations WHERE request_key=$1`, c.Key).Scan(&cancellation); err != nil {
		t.Fatal(err)
	}
	if err = x.repo.CompleteCancellation(ctx, cancellation); err != nil {
		t.Fatal(err)
	}
	var refundID int64
	if err = x.pool.QueryRow(ctx, `SELECT id FROM marketplace_refunds WHERE order_id=$1`, mixed.order.ID).Scan(&refundID); err != nil {
		t.Fatal(err)
	}
	c.Action = "approve_refund"
	c.RefundID = refundID
	c.Reason = "Verified isolated cancellation refund"
	if err = x.repo.Act(ctx, x.admin, c); err != nil {
		t.Fatal(err)
	}
	record, submit, err := x.repo.RefundRecord(ctx, refundID)
	if err != nil || !submit {
		t.Fatalf("refund record: %v %v", submit, err)
	}
	result := ports.PaymentRefund{ProviderRefundID: fmt.Sprintf("isolated-refund-%d", refundID), ProviderPaymentID: record.ProviderPaymentID, AmountCents: record.AmountCents, Currency: record.Currency, Status: "processed"}
	for i := 0; i < 2; i++ {
		if err = x.repo.SavePartialRefund(ctx, refundID, result); err != nil {
			t.Fatal(err)
		}
	}
	return mixed, other
}

func TestCommerceMixedBasketReadScopeAndRefundIntegrity(t *testing.T) {
	x, other := mixedCommerceFixture(t, newMarketplaceFixture(t))
	ctx := context.Background()
	customer, view, requests, err := x.repo.OrderWorkspace(ctx, x.f.userIDs[0], x.order.OrderNumber, ports.RequestQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if len(customer.Items) != 2 || len(view.Lines) != 2 || requests.Total != 1 || customer.Financial.ProviderRefundedCents != 1000 || customer.Financial.DisplayStatus != "partially_refunded" {
		t.Fatalf("mixed customer summary: %+v %+v", customer, requests)
	}
	for _, seller := range []marketplaceFixture{x, other} {
		summary, own, req, err := x.repo.OrderWorkspace(ctx, seller.f.ownerID, x.order.OrderNumber, ports.RequestQuery{})
		if err != nil {
			t.Fatal(err)
		}
		if len(summary.Items) != 1 || len(own.Lines) != 1 || own.Lines[0].SellerID != seller.seller || summary.Financial.CapturedCents != 0 {
			t.Fatal("seller read exposed another seller or customer payment")
		}
		if seller.seller == other.seller && req.Total != 0 {
			t.Fatal("other seller saw a refund for the first seller")
		}
	}
	detail, err := x.commerce.GetOrder(ports.WithOrderWorkspaceMetadata(ctx), x.f.userIDs[0], x.order.OrderNumber)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.ItemProgress) != 2 || len(detail.Requests) != 1 || detail.PaymentSummary.ProviderRefundedCents != 1000 {
		t.Fatal("owned API projection differs from workspace")
	}
}
