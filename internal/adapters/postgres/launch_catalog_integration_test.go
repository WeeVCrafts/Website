package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/wecratfs/commerce/internal/ports"
)

func TestLaunchCatalogCheckoutSharesPilotAndLiveGuards(t *testing.T) {
	pool := openIntegrationPool(t)
	first := newCommerceFixture(t, pool, 1, 5)
	second := newCommerceFixture(t, pool, 1, 5)
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
	for _, fixture := range []commerceFixture{first, second} {
		exec(`UPDATE product_variants SET shipping_weight_g=250,shipping_length_cm=15,shipping_width_cm=10,shipping_height_cm=5,return_class='textile_craft' WHERE id=$1`, fixture.variantID)
		exec(`INSERT INTO marketplace_shipping_profiles(seller_id,pickup_location,postal_code,verified)
			SELECT seller_id,'fixture','500079',true FROM products WHERE id=$1`, fixture.productID)
		exec(`INSERT INTO seller_invoice_profiles(seller_id,legal_name,address,state_code,registration,prefix,verified_by)
			SELECT seller_id,'Fixture seller','Fixture address','36','unregistered','F',$2 FROM products WHERE id=$1`, fixture.productID, fixture.ownerID)
		exec(`INSERT INTO variant_tax_profiles(variant_id,hsn,rate_basis_points,verified_by) VALUES($1,'9701',0,$2)`, fixture.variantID, fixture.ownerID)
	}
	exec(`UPDATE marketplace_policy_settings SET shipping_mode='pilot',published_at=now(),pilot_user_id=$1,pilot_variant_id=$2,pilot_order_id=NULL WHERE id`, first.userIDs[0], first.variantID)
	check := func(label string, want error) {
		t.Helper()
		for _, lock := range []bool{false, true} {
			if err := checkPolicyCheckout(ctx, tx, first.userIDs[0], first.cartIDs[0], lock); !errors.Is(err, want) {
				t.Fatalf("%s, lock=%t: got %v, want %v", label, lock, err, want)
			}
		}
	}
	check("eligible pilot", nil)
	exec(`INSERT INTO cart_items(cart_id,variant_id,quantity) VALUES($1,$2,1)`, first.cartIDs[0], second.variantID)
	check("pilot rejects mixed sellers", ports.ErrDeliveryUnavailable)

	exec(`UPDATE marketplace_policy_settings SET shipping_mode='live' WHERE id`)
	exec(`INSERT INTO marketplace_launch_variants(variant_id) VALUES($1)`, first.variantID)
	check("live rejects unselected line", ports.ErrDeliveryUnavailable)
	exec(`INSERT INTO marketplace_launch_variants(variant_id) VALUES($1)`, second.variantID)
	check("ready launch variants from both sellers", nil)
	exec(`UPDATE marketplace_policy_settings SET live_catalog_scope='seller',
		live_catalog_seller_id=(SELECT seller_id FROM products WHERE id=$1) WHERE id`, first.productID)
	check("seller launch excludes another seller despite its limited launch selection", ports.ErrDeliveryUnavailable)
	exec(`DELETE FROM cart_items WHERE cart_id=$1 AND variant_id=$2`, first.cartIDs[0], second.variantID)
	exec(`DELETE FROM marketplace_launch_variants WHERE variant_id=$1`, first.variantID)
	check("seller launch permits its ready option without limited selection", nil)
	exec(`UPDATE marketplace_policy_settings SET live_catalog_scope='ready',live_catalog_seller_id=NULL WHERE id`)
	exec(`INSERT INTO cart_items(cart_id,variant_id,quantity) VALUES($1,$2,1)`, first.cartIDs[0], second.variantID)
	check("expanded launch permits ready options from both sellers", nil)
	exec(`UPDATE marketplace_policy_settings SET live_catalog_scope='limited' WHERE id`)
	check("restoring limited launch restores eligibility rules", ports.ErrDeliveryUnavailable)
	exec(`INSERT INTO marketplace_launch_variants(variant_id) VALUES($1)`, first.variantID)
	exec(`UPDATE variant_tax_profiles SET rate_basis_points=500 WHERE variant_id=$1`, second.variantID)
	check("live rejects incompatible tax configuration", ports.ErrDeliveryUnavailable)
	exec(`DELETE FROM variant_tax_profiles WHERE variant_id=$1`, second.variantID)
	check("live permits the default when a tax profile is absent", nil)
	exec(`UPDATE inventory_stock SET available_quantity=0 WHERE variant_id=$1`, second.variantID)
	check("live rejects insufficient stock", ports.ErrDeliveryUnavailable)
	exec(`UPDATE marketplace_policy_settings SET shipping_mode='off' WHERE id`)
	check("paused checkout", ports.ErrDeliveryUnavailable)
}
