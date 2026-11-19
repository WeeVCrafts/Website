package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	domain "github.com/wecratfs/commerce/internal/domain/commerce"
	"github.com/wecratfs/commerce/internal/ports"
)

type checkoutProvider struct {
	calls    int
	result   bool
	err      error
	expected ports.Parcel
}

type pricedCheckoutProvider struct {
	checkoutProvider
	rate int64
}

func (p *pricedCheckoutProvider) DeliveryRate(ctx context.Context, origin, destination string, parcel ports.Parcel) (ports.CourierOption, error) {
	available, err := p.DeliveryAvailable(ctx, origin, destination, parcel)
	if err == nil && !available {
		err = ports.ErrDeliveryUnavailable
	}
	return ports.CourierOption{ID: 6, RateCents: p.rate, EstimatedDays: "2"}, err
}

func TestLiveShiprocketCheckoutQuoteAndPlacementAgree(t *testing.T) {
	pool := openIntegrationPool(t)
	ctx := t.Context()
	fixture := newCommerceFixture(t, pool, 1, 2)
	var seller int64
	if err := pool.QueryRow(ctx, `INSERT INTO marketplace_shipping_profiles (seller_id,pickup_location,postal_code,verified)
		SELECT id,'Test Pickup','110030',true FROM sellers WHERE owner_user_id=$1 RETURNING seller_id`, fixture.ownerID).Scan(&seller); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM marketplace_shipping_profiles WHERE seller_id=$1`, seller)
	})
	parcel := ports.Parcel{WeightGrams: 500, LengthCM: 15, WidthCM: 12, HeightCM: 9}
	if _, err := pool.Exec(ctx, `UPDATE product_variants SET shipping_weight_g=500,shipping_length_cm=15,shipping_width_cm=12,shipping_height_cm=9 WHERE id=$1`, fixture.variantID); err != nil {
		t.Fatal(err)
	}
	provider := &pricedCheckoutProvider{checkoutProvider: checkoutProvider{expected: parcel, result: true}, rate: 6543}
	repo := NewCommerceRepository(pool)
	repo.ConfigureCheckoutDelivery(provider, nil, "live")
	user, cart := fixture.userIDs[0], fixture.cartIDs[0]
	address := domain.AddressInput{PostalCode: "411001", CountryCode: "IN"}
	pending, err := repo.QuoteOrder(ctx, user, cart, "")
	if err != nil || !pending.ShippingPending || pending.ShippingCents != 0 || provider.calls != 0 {
		t.Fatalf("pending=%+v calls=%d err=%v", pending, provider.calls, err)
	}
	quote, err := repo.QuoteOrderForDelivery(ctx, user, cart, "", address.PostalCode)
	if err != nil || quote.ShippingPending || quote.ShippingCents != 6543 || quote.TotalCents != 7543 || quote.TransitDays != 2 {
		t.Fatalf("quote=%+v err=%v", quote, err)
	}
	for route, cached := range repo.delivery.cache {
		cached.expires = time.Now().Add(-time.Second)
		repo.delivery.cache[route] = cached
	}
	provider.rate = 5041
	if _, err := repo.CreateQuotedOrder(domain.WithExpectedShipping(ctx, quote.ShippingCents), user, cart, "live-rate-test", address, "", quote.TotalCents); !errors.Is(err, ports.ErrQuoteChanged) {
		t.Fatalf("stale rate placement: %v", err)
	}
	provider.err = ports.ErrShippingUnavailable
	clear(repo.delivery.cache)
	if _, err := repo.QuoteOrderForDelivery(ctx, user, cart, "", address.PostalCode); !errors.Is(err, ports.ErrShippingUnavailable) {
		t.Fatalf("carrier outage quote: %v", err)
	}
	if _, err := repo.CreateQuotedOrder(ctx, user, cart, "live-rate-test", address, "", quote.TotalCents); !errors.Is(err, ports.ErrShippingUnavailable) {
		t.Fatalf("carrier outage placement: %v", err)
	}
	provider.err = nil
	quote, err = repo.QuoteOrderForDelivery(ctx, user, cart, "", address.PostalCode)
	if err != nil || quote.ShippingCents != 5041 || quote.TotalCents != 6041 {
		t.Fatalf("updated quote=%+v err=%v", quote, err)
	}
	order, err := repo.CreateQuotedOrder(domain.WithExpectedShipping(ctx, quote.ShippingCents), user, cart, "live-rate-test", address, "", quote.TotalCents)
	if err != nil || order.ShippingCents != quote.ShippingCents || order.TotalCents != quote.TotalCents {
		t.Fatalf("order=%+v err=%v", order, err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM order_delivery_decisions WHERE order_id=$1`, order.ID)
	})
}

func (p *checkoutProvider) DeliveryAvailable(_ context.Context, origin, destination string, parcel ports.Parcel) (bool, error) {
	p.calls++
	if origin != "110030" || destination != "411001" || parcel != p.expected {
		return false, errors.New("wrong route or estimate")
	}
	return p.result, p.err
}

func TestShiprocketCheckoutPersistsFallbackAndReplaysWithoutProvider(t *testing.T) {
	pool := openIntegrationPool(t)
	ctx := t.Context()
	fixture := newCommerceFixture(t, pool, 1, 2)
	var seller int64
	if err := pool.QueryRow(ctx, `SELECT id FROM sellers WHERE owner_user_id=$1`, fixture.ownerID).Scan(&seller); err != nil {
		t.Fatal(err)
	}
	repo := NewCommerceRepository(pool)
	provider := &checkoutProvider{expected: ports.Parcel{WeightGrams: 550, LengthCM: 17, WidthCM: 14, HeightCM: 11}, result: false}
	repo.ConfigureCheckoutDelivery(provider, map[int64]DeliveryProfile{seller: {PostalCode: "110030", PackingGrams: 50, PackingCM: 2}}, "live")
	address := domain.AddressInput{RecipientName: "Test Buyer", Phone: "9000000000", Line1: "1 Test Lane", City: "Pune", State: "MH", PostalCode: "411001", CountryCode: "IN"}
	user, cart := fixture.userIDs[0], fixture.cartIDs[0]
	if _, err := repo.CreateOrder(ctx, user, cart, "delivery", address); !errors.Is(err, ports.ErrShippingParcelMissing) {
		t.Fatalf("missing parcel: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE product_variants SET shipping_weight_g=500,shipping_length_cm=15,shipping_width_cm=12,shipping_height_cm=9 WHERE id=$1`, fixture.variantID); err != nil {
		t.Fatal(err)
	}
	if err := repo.CheckDelivery(ctx, user, cart, address.PostalCode); !errors.Is(err, ports.ErrDeliveryUnavailable) {
		t.Fatalf("negative quote: %v", err)
	}
	if _, err := repo.CreateOrder(ctx, user, cart, "delivery", address); !errors.Is(err, ports.ErrDeliveryUnavailable) {
		t.Fatalf("negative placement: %v", err)
	}
	if provider.calls != 1 {
		t.Fatalf("negative serviceability calls: %d", provider.calls)
	}
	clear(repo.delivery.cache)
	provider.err = ports.ErrShippingUnavailable
	order, err := repo.CreateOrder(ctx, user, cart, "delivery", address)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM delivery_exceptions WHERE order_id=$1`, order.ID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM order_delivery_decisions WHERE order_id=$1`, order.ID)
	})
	var decision string
	var fallback bool
	if err := pool.QueryRow(ctx, `SELECT d.decision,EXISTS(SELECT 1 FROM delivery_exceptions e WHERE e.order_id=d.order_id AND e.seller_id=d.seller_id AND e.kind='checkout_fallback')
 FROM order_delivery_decisions d WHERE d.order_id=$1 AND d.seller_id=$2`, order.ID, seller).Scan(&decision, &fallback); err != nil || decision != "manual_fallback" || !fallback {
		t.Fatalf("decision=%s fallback=%t err=%v", decision, fallback, err)
	}
	replay, err := repo.CreateOrder(ctx, user, cart, "delivery", address)
	if err != nil || replay.ID != order.ID || provider.calls != 2 {
		t.Fatalf("saved order replay: id=%d calls=%d err=%v", replay.ID, provider.calls, err)
	}
}

func TestShiprocketPositiveCheckoutCache(t *testing.T) {
	parcel := ports.Parcel{WeightGrams: 100, LengthCM: 15, WidthCM: 12, HeightCM: 8}
	provider := &checkoutProvider{expected: parcel, result: true}
	repo := NewCommerceRepository(nil)
	repo.ConfigureCheckoutDelivery(provider, map[int64]DeliveryProfile{4: {PostalCode: "110030"}}, "live")
	lines := []deliveryLine{{variant: 8, seller: 4, quantity: 1, parcel: parcel}, {variant: 9, seller: 5, quantity: 1}}
	for i := 0; i < 2; i++ {
		decisions, err := repo.delivery.check(t.Context(), lines, "411001")
		if err != nil || len(decisions) != 2 || decisions[0].method != "shiprocket" || decisions[1].method != "manual" {
			t.Fatalf("decisions=%+v err=%v", decisions, err)
		}
	}
	if provider.calls != 1 {
		t.Fatalf("positive cache calls=%d", provider.calls)
	}
}
