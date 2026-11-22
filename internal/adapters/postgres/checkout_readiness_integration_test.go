package postgres

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	domain "github.com/wecratfs/commerce/internal/domain/commerce"
	"github.com/wecratfs/commerce/internal/ports"
)

func TestAutomaticCheckoutUsesFreshReadinessForEveryBuyer(t *testing.T) {
	pool := openIntegrationPool(t)
	fixture := newCommerceFixture(t, pool, 2, 5)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := tx.Exec(ctx, query, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(`UPDATE product_variants SET shipping_weight_g=250,shipping_length_cm=15,shipping_width_cm=10,shipping_height_cm=5 WHERE id=$1`, fixture.variantID)
	exec(`INSERT INTO marketplace_shipping_profiles(seller_id,pickup_location,postal_code,verified)
		SELECT seller_id,'fixture','500079',true FROM products WHERE id=$1`, fixture.productID)
	exec(`INSERT INTO seller_invoice_profiles(seller_id,legal_name,address,state_code,registration,prefix,verified_by)
		SELECT seller_id,'Fixture seller','Fixture address','36','unregistered','F',$2 FROM products WHERE id=$1`, fixture.productID, fixture.ownerID)
	exec(`UPDATE marketplace_policy_settings SET shipping_mode='off',published_at=now(),pilot_order_id=NULL WHERE id`)
	exec(`INSERT INTO marketplace_checkout_health(id,checked_at,valid_until,blockers,pickup_postcodes)
		VALUES(true,now(),now()+interval '5 minutes','{}','{"fixture":"500079"}')`)
	check := func(want error) {
		t.Helper()
		for i, user := range fixture.userIDs {
			for _, lock := range []bool{false, true} {
				if err := checkPolicyCheckout(ctx, tx, user, fixture.cartIDs[i], lock); !errors.Is(err, want) {
					t.Fatalf("buyer %d, lock=%t: got %v, want %v", i, lock, err, want)
				}
			}
		}
	}
	check(nil)
	exec(`UPDATE marketplace_checkout_health SET valid_until=now() WHERE id`)
	check(ports.ErrDeliveryUnavailable)
	exec(`UPDATE marketplace_checkout_health SET valid_until=now()+interval '5 minutes', blockers=ARRAY['shiprocket_wallet'] WHERE id`)
	check(ports.ErrDeliveryUnavailable)
	exec(`UPDATE marketplace_checkout_health SET blockers='{}' WHERE id`)
	check(nil)
	exec(`UPDATE marketplace_policy_settings SET checkout_paused=true WHERE id`)
	check(ports.ErrDeliveryUnavailable)
	exec(`UPDATE marketplace_policy_settings SET checkout_paused=false WHERE id`)
	exec(`UPDATE marketplace_shipping_profiles SET postal_code='500080' WHERE seller_id=(SELECT seller_id FROM products WHERE id=$1)`, fixture.productID)
	check(ports.ErrDeliveryUnavailable)
}

func TestBlockedAutomaticCheckoutPreservesCartAndNotifiesOnceWhenReady(t *testing.T) {
	if !strings.Contains(os.Getenv("DATABASE_URL"), "@127.0.0.1:5432/weevcrafts_automatic_checkout_") {
		t.Skip("a disposable automatic checkout database is required")
	}
	pool := openIntegrationPool(t)
	fixture := newCommerceFixture(t, pool, 2, 5)
	ctx, cancel := context.WithTimeout(context.Background(), 75*time.Second)
	defer cancel()
	var seller int64
	if err := pool.QueryRow(ctx, `SELECT seller_id FROM products WHERE id=$1`, fixture.productID).Scan(&seller); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		for _, query := range []string{
			`DELETE FROM marketplace_checkout_health`, `DELETE FROM marketplace_checkout_issue`,
			`UPDATE marketplace_policy_settings SET shipping_mode='off',live_catalog_scope='limited',published_at=NULL,checkout_paused=false WHERE id`,
		} {
			if _, err := pool.Exec(context.Background(), query); err != nil {
				t.Error(err)
			}
		}
	})
	for _, step := range []struct {
		query string
		args  []any
	}{
		{`UPDATE product_variants SET shipping_weight_g=250,shipping_length_cm=15,shipping_width_cm=10,shipping_height_cm=5 WHERE id=$1`, []any{fixture.variantID}},
		{`INSERT INTO marketplace_shipping_profiles(seller_id,pickup_location,postal_code,verified) VALUES($1,'fixture','500079',true)`, []any{seller}},
		{`INSERT INTO seller_invoice_profiles(seller_id,legal_name,address,state_code,registration,prefix,verified_by) VALUES($1,'Fixture seller','Fixture address','36','unregistered','F',$2)`, []any{seller, fixture.ownerID}},
		{`UPDATE marketplace_policy_settings SET published_at=now(),shipping_mode='off' WHERE id`, nil},
	} {
		if _, err := pool.Exec(ctx, step.query, step.args...); err != nil {
			t.Fatal(err)
		}
	}
	market := NewMarketplaceRepository(pool)
	if err := market.InitializeAutomaticCheckout(ctx, "fixture"); err != nil {
		t.Fatal(err)
	}
	repo := NewCommerceRepository(pool)
	repo.RequirePolicyCheckout(true)
	address := domain.AddressInput{RecipientName: "Fixture", Phone: "9000000000", Line1: "Test lane", City: "Pune", State: "MH", PostalCode: "411001", CountryCode: "IN"}
	_, err := repo.CreateOrder(ctx, fixture.userIDs[0], fixture.cartIDs[0], "blocked-checkout-fixture", address)
	if !errors.Is(err, ports.ErrDeliveryUnavailable) {
		t.Fatalf("blocked order: %v", err)
	}
	var orders, attempts, available, reserved, waiting int
	err = pool.QueryRow(ctx, `SELECT
		(SELECT count(*) FROM orders WHERE user_id=ANY($1)),
		(SELECT count(*) FROM payment_attempts p JOIN orders o ON o.id=p.order_id WHERE o.user_id=ANY($1)),
		(SELECT available_quantity FROM inventory_stock WHERE variant_id=$2),
		(SELECT reserved_quantity FROM inventory_stock WHERE variant_id=$2),
		(SELECT count(*) FROM marketplace_checkout_wait WHERE cart_id=ANY($3))`, fixture.userIDs, fixture.variantID, fixture.cartIDs).
		Scan(&orders, &attempts, &available, &reserved, &waiting)
	if err != nil || orders != 0 || attempts != 0 || available != 5 || reserved != 0 || waiting != 1 {
		t.Fatalf("blocked checkout side effects: orders=%d attempts=%d available=%d reserved=%d waiting=%d err=%v", orders, attempts, available, reserved, waiting, err)
	}
	if err := market.SaveCheckoutHealth(ctx, "fixture", time.Now(), nil, map[string]string{"fixture": "500079"}); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := market.PublishCheckoutGuidance(ctx); err != nil {
			t.Fatal(err)
		}
	}
	for i, user := range fixture.userIDs {
		var count int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM notifications WHERE user_id=$1 AND source_key LIKE 'checkout-ready:%'`, user).Scan(&count); err != nil || count != 1-i {
			t.Fatalf("buyer %d: got %d checkout notifications: %v", i, count, err)
		}
		if _, err := repo.CreateOrder(ctx, user, fixture.cartIDs[i], fmt.Sprintf("checkout-ready-fixture-%d", user), address); err != nil {
			t.Fatalf("eligible buyer %d: %v", i, err)
		}
	}
}
