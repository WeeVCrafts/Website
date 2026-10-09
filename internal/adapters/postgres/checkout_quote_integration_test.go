package postgres

import (
	"context"
	"errors"
	"fmt"
	"testing"

	domaincommerce "github.com/wecratfs/commerce/internal/domain/commerce"
	"github.com/wecratfs/commerce/internal/ports"
)

func TestCheckoutRejectsForeignAndExpiredCartWithoutFallback(t *testing.T) {
	pool := openIntegrationPool(t)
	f := newCommerceFixture(t, pool, 2, 10)
	r := NewCommerceRepository(pool)
	ctx := context.Background()
	address := domaincommerce.AddressInput{RecipientName: "QA", Phone: "9000000000", Line1: "Test lane", City: "Pune", State: "MH", PostalCode: "411001", CountryCode: "IN"}
	if _, err := r.CreateOrder(ctx, f.userIDs[0], f.cartIDs[1], "foreign-cart-check", address); !errors.Is(err, ports.ErrCartNotFound) {
		t.Fatalf("foreign cart checkout error=%v; want cart not found", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE carts SET expires_at=NOW()-INTERVAL '1 minute' WHERE id=$1`, f.cartIDs[0]); err != nil {
		t.Fatal(err)
	}
	replacement, err := r.GetOrCreateCart(ctx, f.userIDs[0], "")
	if err != nil {
		t.Fatal(err)
	}
	if replacement.ID == f.cartIDs[0] {
		t.Fatal("expired cart was reused")
	}
	if err := r.AddCartItem(ctx, replacement.ID, f.variantID, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := r.CreateOrder(ctx, f.userIDs[0], f.cartIDs[0], "expired-cart-check", address); !errors.Is(err, ports.ErrCartNotFound) {
		t.Fatalf("expired cart checkout error=%v; want cart not found", err)
	}
	var orders, reserved int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM orders WHERE user_id=ANY($1)`, f.userIDs).Scan(&orders); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT reserved_quantity FROM inventory_stock WHERE variant_id=$1`, f.variantID).Scan(&reserved); err != nil {
		t.Fatal(err)
	}
	if orders != 0 || reserved != 0 {
		t.Fatalf("denied checkout created orders=%d reserved=%d", orders, reserved)
	}
}

func TestCouponQuoteMatchesOrderAndRetryConsumesOnce(t *testing.T) {
	pool := openIntegrationPool(t)
	f := newCommerceFixture(t, pool, 1, 10)
	r := NewCommerceRepository(pool)
	ctx := context.Background()
	var promotionID, couponID int64
	code := fmt.Sprintf("QA-%d", f.variantID)
	if err := pool.QueryRow(ctx, `INSERT INTO promotions(slug,name,discount_type,discount_value,status,ends_at) VALUES($1,'QA coupon','percentage',10,'active',NOW()+INTERVAL '1 hour') RETURNING id`, code).Scan(&promotionID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM promotion_usages WHERE promotion_id=$1`, promotionID)
		_, _ = pool.Exec(ctx, `DELETE FROM promotions WHERE id=$1`, promotionID)
	})
	if err := pool.QueryRow(ctx, `INSERT INTO coupons(promotion_id,code,usage_limit) VALUES($1,$2,1) RETURNING id`, promotionID, code).Scan(&couponID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE cart_items SET quantity=2 WHERE cart_id=$1`, f.cartIDs[0]); err != nil {
		t.Fatal(err)
	}
	quote, err := r.QuoteOrder(ctx, f.userIDs[0], f.cartIDs[0], code)
	if err != nil || quote.ItemCount != 2 || quote.DiscountCents != 200 || quote.TotalCents != 11700 {
		t.Fatalf("coupon quote=%+v error=%v", quote, err)
	}
	address := domaincommerce.AddressInput{RecipientName: "QA", Phone: "9000000000", Line1: "Test lane", City: "Pune", State: "MH", PostalCode: "411001", CountryCode: "IN"}
	order, err := r.CreateQuotedOrder(ctx, f.userIDs[0], f.cartIDs[0], "coupon-retry-check", address, code, quote.TotalCents)
	if err != nil || order.TotalCents != quote.TotalCents || order.DiscountCents != quote.DiscountCents {
		t.Fatalf("coupon order=%+v error=%v", order, err)
	}
	retry, err := r.CreateQuotedOrder(ctx, f.userIDs[0], f.cartIDs[0], "coupon-retry-check", address, code, quote.TotalCents)
	if err != nil || retry.ID != order.ID {
		t.Fatalf("coupon retry=%+v error=%v", retry, err)
	}
	var couponUses, promotionUses, usages int
	if err := pool.QueryRow(ctx, `SELECT c.usage_count,p.usage_count,(SELECT COUNT(*) FROM promotion_usages WHERE coupon_id=c.id) FROM coupons c JOIN promotions p ON p.id=c.promotion_id WHERE c.id=$1`, couponID).Scan(&couponUses, &promotionUses, &usages); err != nil {
		t.Fatal(err)
	}
	if couponUses != 1 || promotionUses != 1 || usages != 1 {
		t.Fatalf("coupon uses=%d promotion uses=%d redemptions=%d; want once each", couponUses, promotionUses, usages)
	}
}

func TestCheckoutQuoteRejectsChangedPriceWithoutReservingStock(t *testing.T) {
	pool := openIntegrationPool(t)
	f := newCommerceFixture(t, pool, 2, 10)
	r := NewCommerceRepository(pool)
	ctx := context.Background()
	quote, err := r.QuoteOrder(ctx, f.userIDs[0], f.cartIDs[0], "")
	if err != nil || quote.TotalCents != 10900 {
		t.Fatalf("QuoteOrder()=%+v,%v; want total 10900", quote, err)
	}
	if _, err = r.QuoteOrder(ctx, f.userIDs[1], f.cartIDs[0], ""); !errors.Is(err, ports.ErrCartNotFound) {
		t.Fatalf("foreign cart quote error=%v; want cart not found", err)
	}
	if _, err = pool.Exec(ctx, `UPDATE product_variants SET price_cents=2000 WHERE id=$1`, f.variantID); err != nil {
		t.Fatal(err)
	}
	address := domaincommerce.AddressInput{RecipientName: "QA", Phone: "9000000000", Line1: "Test lane", City: "Pune", State: "MH", PostalCode: "411001", CountryCode: "IN"}
	_, err = r.CreateQuotedOrder(ctx, f.userIDs[0], f.cartIDs[0], "quote-change-check-123", address, "", quote.TotalCents)
	if !errors.Is(err, ports.ErrQuoteChanged) {
		t.Fatalf("CreateQuotedOrder() error=%v; want quote changed", err)
	}
	var available, reserved, orders int
	if err = pool.QueryRow(ctx, `SELECT available_quantity,reserved_quantity FROM inventory_stock WHERE variant_id=$1`, f.variantID).Scan(&available, &reserved); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `SELECT COUNT(*) FROM orders WHERE user_id=$1`, f.userIDs[0]).Scan(&orders); err != nil {
		t.Fatal(err)
	}
	if available != 10 || reserved != 0 || orders != 0 {
		t.Fatalf("rejected quote changed stock/order state: available=%d reserved=%d orders=%d", available, reserved, orders)
	}
	quote, err = r.QuoteOrder(ctx, f.userIDs[0], f.cartIDs[0], "")
	if err != nil {
		t.Fatal(err)
	}
	order, err := r.CreateQuotedOrder(ctx, f.userIDs[0], f.cartIDs[0], "quote-change-check-123", address, "", quote.TotalCents)
	if err != nil || order.TotalCents != quote.TotalCents {
		t.Fatalf("confirmed fresh quote order=%+v err=%v", order, err)
	}
	var eventID int64
	if err = pool.QueryRow(ctx, `SELECT id FROM outbox_events WHERE event_type='OrderCreated' AND aggregate_id=$1`, order.OrderNumber).Scan(&eventID); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if _, _, err = r.PrepareOrderConfirmation(ctx, eventID, order.OrderNumber); err != nil {
			t.Fatal(err)
		}
	}
	var notices int
	if err = pool.QueryRow(ctx, `SELECT COUNT(*) FROM notifications WHERE source_event_id=$1`, eventID).Scan(&notices); err != nil {
		t.Fatal(err)
	}
	if notices != 1 {
		t.Fatalf("confirmation notifications=%d; want 1", notices)
	}
}
