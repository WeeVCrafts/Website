package postgres

import (
	"context"
	"fmt"
	domain "github.com/wecratfs/commerce/internal/domain/commerce"
	"github.com/wecratfs/commerce/internal/ports"
	"sync"
	"testing"
	"time"
)

func TestCreditFundedCancellationRefundsOriginalFundingOnly(t *testing.T) {
	for _, grantAmount := range []int64{500, 50000} {
		t.Run(fmt.Sprint(grantAmount), func(t *testing.T) {
			x := newMarketplaceFixture(t)
			ctx := domain.WithStoreCredit(context.Background(), true)
			user := x.f.userIDs[1]
			if _, err := x.pool.Exec(ctx, `INSERT INTO store_credit_grants(user_id,amount_cents,remaining_cents,expires_at) VALUES($1,$2,$2,now()+interval '6 months')`, user, grantAmount); err != nil {
				t.Fatal(err)
			}
			o, err := x.commerce.CreateOrder(ctx, user, x.f.cartIDs[1], fmt.Sprintf("funding-cancel-%d", time.Now().UnixNano()), domain.AddressInput{RecipientName: "Fixture buyer", Phone: "9000000000", Line1: "Fixture lane", City: "Pune", State: "MH", PostalCode: "411001", CountryCode: "IN"})
			if err != nil {
				t.Fatal(err)
			}
			cash := o.TotalCents - o.CreditCents
			if cash > 0 {
				p := NewPaymentRepository(x.pool)
				provider := fmt.Sprintf("funding-order-%d", o.ID)
				if _, err = p.AttachProviderOrder(ctx, user, o.OrderNumber, ports.PaymentOrder{ProviderOrderID: provider, AmountCents: cash, Currency: "INR"}); err != nil {
					t.Fatal(err)
				}
				if _, err = p.ProcessWebhook(ctx, ports.PaymentWebhook{EventID: provider + "-capture", EventType: "payment.captured", ProviderOrderID: provider, ProviderPaymentID: provider + "-payment", AmountCents: cash, Currency: "INR"}, []byte(`{}`)); err != nil {
					t.Fatal(err)
				}
			}
			v, err := x.repo.View(ctx, user, o.OrderNumber)
			if err != nil {
				t.Fatal(err)
			}
			c := ports.MarketplaceCommand{Action: "cancel", Number: o.OrderNumber, SellerID: x.seller, ItemID: v.Lines[0].ID, Quantity: 1, Reason: "Fixture cancellation before dispatch", Key: fmt.Sprintf("cancel-funding-%d", o.ID)}
			if err = x.repo.Act(ctx, user, c); err != nil {
				t.Fatal(err)
			}
			var cancellation int64
			if err = x.pool.QueryRow(ctx, `SELECT id FROM marketplace_cancellations WHERE request_key=$1`, c.Key).Scan(&cancellation); err != nil {
				t.Fatal(err)
			}
			if err = x.repo.CompleteCancellation(ctx, cancellation); err != nil {
				t.Fatal(err)
			}
			c.Action = "approve_refund"
			if err = x.pool.QueryRow(ctx, `SELECT id FROM marketplace_refunds WHERE order_id=$1`, o.ID).Scan(&c.RefundID); err != nil {
				t.Fatal(err)
			}
			if err = x.repo.Act(ctx, x.admin, c); err != nil {
				t.Fatal(err)
			}
			record, submit, err := x.repo.RefundRecord(ctx, c.RefundID)
			if err != nil || !submit || record.AmountCents != cash {
				t.Fatalf("cash refund mismatch %+v %v", record, err)
			}
			refundID := ""
			if cash > 0 {
				refundID = fmt.Sprintf("fixture-refund-%d", o.ID)
			}
			result := ports.PaymentRefund{ProviderPaymentID: record.ProviderPaymentID, ProviderRefundID: refundID, AmountCents: cash, Currency: "INR", Status: "processed"}
			for i := 0; i < 2; i++ {
				if err = x.repo.SavePartialRefund(ctx, c.RefundID, result); err != nil {
					t.Fatal(err)
				}
			}
			available, err := creditAvailable(ctx, x.pool, user)
			if err != nil || available != grantAmount {
				t.Fatalf("credit duplicated or lost: %d want %d %v", available, grantAmount, err)
			}
			var state string
			if err = x.pool.QueryRow(ctx, `SELECT status FROM orders WHERE id=$1`, o.ID).Scan(&state); err != nil || state != "refunded" {
				t.Fatalf("refund not complete %s %v", state, err)
			}
		})
	}
}

func TestPilotAdmitsOneOrderAcrossConcurrentCarts(t *testing.T) {
	x := newMarketplaceFixture(t)
	ctx := context.Background()
	user := x.f.userIDs[1]
	if _, err := x.pool.Exec(ctx, `UPDATE product_variants SET shipping_weight_g=500,shipping_length_cm=15,shipping_width_cm=12,shipping_height_cm=8,return_class='textile_craft' WHERE id=$1`, x.f.variantID); err != nil {
		t.Fatal(err)
	}
	if _, err := x.pool.Exec(ctx, `UPDATE marketplace_policy_settings SET published_at=now(),shipping_mode='pilot',pilot_user_id=$1,pilot_variant_id=$2,pilot_order_id=NULL WHERE id`, user, x.f.variantID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = x.pool.Exec(context.Background(), `UPDATE marketplace_policy_settings SET published_at=NULL,shipping_mode='off',pilot_user_id=NULL,pilot_variant_id=NULL,pilot_order_id=NULL WHERE id`)
	})
	intent, err := x.commerce.CreateCheckoutIntent(ctx, user, x.f.variantID, 1, fmt.Sprintf("pilot-extra-cart-%d", x.order.ID))
	if err != nil {
		t.Fatal(err)
	}
	extraCart := intent.CartID

	x.commerce.RequirePolicyCheckout(true)
	x.commerce.RequireInvoiceProfiles(true)
	var group sync.WaitGroup
	results := make(chan error, 2)
	for i, cart := range []int64{x.f.cartIDs[1], extraCart} {
		group.Add(1)
		go func(i int, cart int64) {
			defer group.Done()
			_, err := x.commerce.CreateOrder(ctx, user, cart, fmt.Sprintf("pilot-race-%d-%d", x.order.ID, i), domain.AddressInput{RecipientName: "Fixture buyer", Phone: "9000000000", Line1: "Fixture lane", City: "Pune", State: "MH", PostalCode: "411001", CountryCode: "IN"})
			results <- err
		}(i, cart)
	}
	group.Wait()
	close(results)
	passed := 0
	for err := range results {
		if err == nil {
			passed++
		} else if err != ports.ErrDeliveryUnavailable {
			t.Fatalf("unexpected race failure %v", err)
		}
	}
	if passed != 1 {
		t.Fatalf("pilot admitted %d purchases", passed)
	}
}
