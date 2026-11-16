package postgres

import (
	"context"
	"testing"

	"github.com/wecratfs/commerce/internal/ports"
)

type recoveryEmailRecorder struct{ messages []ports.EmailMessage }

func (r *recoveryEmailRecorder) Send(_ context.Context, message ports.EmailMessage) error {
	r.messages = append(r.messages, message)
	return nil
}

func TestCheckoutRecoveryRequiresConsentStockAndNoPayment(t *testing.T) {
	x := newMarketplaceFixture(t)
	ctx := context.Background()
	var mode string
	if err := x.pool.QueryRow(ctx, `SELECT shipping_mode FROM marketplace_policy_settings WHERE id`).Scan(&mode); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = x.pool.Exec(ctx, `UPDATE marketplace_policy_settings SET shipping_mode=$1 WHERE id`, mode)
	})
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := x.pool.Exec(ctx, query, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(`UPDATE marketplace_policy_settings SET shipping_mode='live',published_at=now() WHERE id`)
	exec(`UPDATE product_variants SET shipping_weight_g=250,shipping_length_cm=15,shipping_width_cm=10,shipping_height_cm=5,return_class='textile_craft' WHERE id=$1`, x.f.variantID)
	exec(`INSERT INTO marketplace_launch_variants(variant_id) VALUES($1)`, x.f.variantID)
	exec(`INSERT INTO checkout_recovery(cart_id,first_quoted_at,last_quoted_at) VALUES($1,now()-interval '2 hours',now()-interval '2 hours')`, x.f.cartIDs[1])
	exec(`INSERT INTO marketing_preferences(user_id,email_marketing) VALUES($1,false)`, x.f.userIDs[1])
	sender := &recoveryEmailRecorder{}
	run := func(want int) {
		t.Helper()
		if err := x.repo.RecoverCheckout(ctx, sender, "https://shop.example.invalid"); err != nil {
			t.Fatal(err)
		}
		if len(sender.messages) != want {
			t.Fatalf("sent %d messages, want %d", len(sender.messages), want)
		}
	}
	run(0)
	exec(`UPDATE marketing_preferences SET email_marketing=true WHERE user_id=$1`, x.f.userIDs[1])
	exec(`UPDATE inventory_stock SET available_quantity=0 WHERE variant_id=$1`, x.f.variantID)
	run(0)
	exec(`UPDATE inventory_stock SET available_quantity=10 WHERE variant_id=$1`, x.f.variantID)
	// A pending order is sufficient to suppress a reminder.
	exec(`UPDATE checkout_recovery SET order_id=$2 WHERE cart_id=$1`, x.f.cartIDs[1], x.order.ID)
	run(0)
	exec(`UPDATE checkout_recovery SET order_id=NULL WHERE cart_id=$1`, x.f.cartIDs[1])
	run(1)
	run(1)
	if sender.messages[0].IdempotencyKey == "" || sender.messages[0].ActionURL != "https://shop.example.invalid/cart" {
		t.Fatal("recovery message requires a stable key and the owned cart entry point")
	}
}
