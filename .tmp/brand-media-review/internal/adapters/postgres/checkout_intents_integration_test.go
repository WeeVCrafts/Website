package postgres

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	domaincommerce "github.com/wecratfs/commerce/internal/domain/commerce"
	"github.com/wecratfs/commerce/internal/ports"
)

func TestBuyNowPreservesShoppingCartAndReservesOnlyPurchasedQuantity(t *testing.T) {
	pool := openIntegrationPool(t)
	f := newCommerceFixture(t, pool, 2, 20)
	ctx := context.Background()
	r := NewCommerceRepository(pool)
	if _, err := pool.Exec(ctx, `INSERT INTO user_roles(user_id,role_slug) VALUES($1,'customer'),($2,'customer')`, f.userIDs[0], f.userIDs[1]); err != nil {
		t.Fatal(err)
	}
	if err := r.UpdateCartItem(ctx, f.cartIDs[0], f.variantID, 3); err != nil {
		t.Fatal(err)
	}
	intent, err := r.CreateCheckoutIntent(ctx, f.userIDs[0], f.variantID, 2, "buy-now-isolated-test")
	if err != nil {
		t.Fatal(err)
	}
	if len(intent.ID) != 48 || time.Until(intent.ExpiresAt) < 23*time.Hour {
		t.Fatalf("invalid context: %+v", intent)
	}
	if _, err := r.GetCheckoutIntent(ctx, f.userIDs[1], intent.ID); !errors.Is(err, ports.ErrCartNotFound) {
		t.Fatalf("foreign context: %v", err)
	}
	var reserved int
	if err := pool.QueryRow(ctx, `SELECT reserved_quantity FROM inventory_stock WHERE variant_id=$1`, f.variantID).Scan(&reserved); err != nil || reserved != 0 {
		t.Fatalf("starting checkout reserved stock: %d, %v", reserved, err)
	}
	quote, err := r.QuoteOrder(ctx, f.userIDs[0], intent.CartID, "")
	if err != nil || quote.ItemCount != 2 || quote.SubtotalCents != 2000 {
		t.Fatalf("quote=%+v err=%v", quote, err)
	}
	address := domaincommerce.AddressInput{RecipientName: "QA", Phone: "9000000000", Line1: "Test lane", City: "Pune", State: "MH", PostalCode: "411001", CountryCode: "IN"}
	order, err := r.CreateQuotedOrder(ctx, f.userIDs[0], intent.CartID, "buy-now-order-retry", address, "", quote.TotalCents)
	if err != nil {
		t.Fatal(err)
	}
	retry, err := r.CreateQuotedOrder(ctx, f.userIDs[0], intent.CartID, "buy-now-order-retry", address, "", quote.TotalCents)
	if err != nil || retry.ID != order.ID {
		t.Fatalf("duplicate order: %+v %v", retry, err)
	}
	saved, err := r.GetCheckoutIntent(ctx, f.userIDs[0], intent.ID)
	if err != nil || saved.OrderNumber != order.OrderNumber {
		t.Fatalf("saved context=%+v %v", saved, err)
	}
	normal, err := r.GetOrCreateCart(ctx, f.userIDs[0], "")
	if err != nil || normal.ID != f.cartIDs[0] || normal.ItemCount != 3 {
		t.Fatalf("normal cart changed: %+v %v", normal, err)
	}
	if err := pool.QueryRow(ctx, `SELECT reserved_quantity FROM inventory_stock WHERE variant_id=$1`, f.variantID).Scan(&reserved); err != nil || reserved != 2 {
		t.Fatalf("wrong reserved amount: %d %v", reserved, err)
	}
}

func TestBuyNowConcurrentRequestsAreIsolatedAndExpire(t *testing.T) {
	pool := openIntegrationPool(t)
	f := newCommerceFixture(t, pool, 1, 20)
	ctx := context.Background()
	r := NewCommerceRepository(pool)
	if _, err := pool.Exec(ctx, `INSERT INTO user_roles(user_id,role_slug) VALUES($1,'customer')`, f.userIDs[0]); err != nil {
		t.Fatal(err)
	}
	var group sync.WaitGroup
	results := make(chan domaincommerce.CheckoutIntent, 6)
	failures := make(chan error, 6)
	for i := 0; i < 6; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			intent, err := r.CreateCheckoutIntent(ctx, f.userIDs[0], f.variantID, 2, "same-buy-now-request")
			results <- intent
			failures <- err
		}()
	}
	group.Wait()
	close(results)
	close(failures)
	for err := range failures {
		if err != nil {
			t.Fatal(err)
		}
	}
	var token string
	var cartID int64
	for intent := range results {
		if token != "" && token != intent.ID {
			t.Fatal("duplicate purchase contexts")
		}
		token = intent.ID
		cartID = intent.CartID
	}
	if _, err := r.CreateCheckoutIntent(ctx, f.userIDs[0], f.variantID, 3, "same-buy-now-request"); !errors.Is(err, ports.ErrIdempotencyConflict) {
		t.Fatalf("changed retry=%v", err)
	}
	other, err := r.CreateCheckoutIntent(ctx, f.userIDs[0], f.variantID, 1, "other-tab-buy-request")
	if err != nil || other.ID == token {
		t.Fatalf("other tab not isolated: %+v %v", other, err)
	}
	if _, err := pool.Exec(ctx, `UPDATE carts SET expires_at=NOW()-INTERVAL '1 minute' WHERE id=$1`, cartID); err != nil {
		t.Fatal(err)
	}
	if _, err := r.GetCheckoutIntent(ctx, f.userIDs[0], token); !errors.Is(err, ports.ErrCartNotFound) {
		t.Fatalf("expired context=%v", err)
	}
	if _, err := r.CreateCheckoutIntent(ctx, f.userIDs[0], f.variantID, 99, "insufficient-buy-now"); !errors.Is(err, ports.ErrInsufficientStock) {
		t.Fatalf("stock=%v", err)
	}
}
