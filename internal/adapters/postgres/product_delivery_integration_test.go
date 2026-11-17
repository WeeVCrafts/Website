package postgres

import (
	"context"
	"testing"

	"github.com/wecratfs/commerce/internal/ports"
)

type productDeliveryProvider struct {
	calls  int
	parcel ports.Parcel
}

func (p *productDeliveryProvider) DeliveryEstimate(_ context.Context, pickup, pin string, parcel ports.Parcel) (bool, int, error) {
	p.calls++
	p.parcel = parcel
	if pickup != "500079" || pin != "110030" {
		return false, 0, ports.ErrShippingInput
	}
	return true, 4, nil
}

func TestProductDeliveryReadsVerifiedParcelWithoutCreatingOrder(t *testing.T) {
	pool := openIntegrationPool(t)
	f := newCommerceFixture(t, pool, 1, 5)
	ctx := t.Context()
	var seller int64
	var slug string
	if err := pool.QueryRow(ctx, `SELECT p.seller_id,p.slug FROM products p JOIN product_variants v ON v.product_id=p.id WHERE v.id=$1`, f.variantID).Scan(&seller, &slug); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO marketplace_shipping_profiles(seller_id,pickup_location,postal_code,verified) VALUES($1,'Fixture','500079',true)`, seller); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM marketplace_shipping_profiles WHERE seller_id=$1`, seller)
	})
	var ordersBefore, reservedBefore int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM orders WHERE user_id=$1`, f.userIDs[0]).Scan(&ordersBefore)
	_ = pool.QueryRow(ctx, `SELECT reserved_quantity FROM inventory_stock WHERE variant_id=$1`, f.variantID).Scan(&reservedBefore)
	provider := &productDeliveryProvider{}
	repo := NewProductDeliveryRepository(pool, provider, map[int64]DeliveryProfile{seller: {PackingGrams: 50, PackingCM: 2}})
	result, err := repo.ProductDelivery(ctx, slug, f.variantID, 2, "110030")
	if err != nil || result.State != "unavailable" || provider.calls != 0 {
		t.Fatalf("missing parcel accepted: %+v %v", result, err)
	}
	if _, err := pool.Exec(ctx, `UPDATE product_variants SET shipping_weight_g=500,shipping_length_cm=15,shipping_width_cm=12,shipping_height_cm=9 WHERE id=$1`, f.variantID); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		result, err = repo.ProductDelivery(ctx, slug, f.variantID, 2, "110030")
		if err != nil || result.State != "available" || result.TransitDays != 4 {
			t.Fatalf("delivery %+v err=%v", result, err)
		}
	}
	want := ports.Parcel{WeightGrams: 1050, LengthCM: 17, WidthCM: 14, HeightCM: 20}
	if provider.calls != 1 || provider.parcel != want {
		t.Fatalf("calls=%d parcel=%+v", provider.calls, provider.parcel)
	}
	var ordersAfter, reservedAfter int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM orders WHERE user_id=$1`, f.userIDs[0]).Scan(&ordersAfter)
	_ = pool.QueryRow(ctx, `SELECT reserved_quantity FROM inventory_stock WHERE variant_id=$1`, f.variantID).Scan(&reservedAfter)
	if ordersBefore != ordersAfter || reservedBefore != reservedAfter {
		t.Fatal("read-only delivery check changed orders or reserved inventory")
	}
	if _, err := pool.Exec(ctx, `UPDATE marketplace_shipping_profiles SET verified=false WHERE seller_id=$1`, seller); err != nil {
		t.Fatal(err)
	}
	result, err = repo.ProductDelivery(ctx, slug, f.variantID, 2, "110030")
	if err != nil || result.State != "unavailable" || provider.calls != 1 {
		t.Fatal("cached delivery bypassed pickup verification")
	}
}
