package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/wecratfs/commerce/internal/domain/commerce"
	"github.com/wecratfs/commerce/internal/ports"
)

func TestSellerDraftCanStartWithReusableOrMeasuredParcel(t *testing.T) {
	x := newMarketplaceFixture(t)
	ctx := t.Context()
	catalog := NewCommerceRepository(x.pool)
	input := commerce.ProductDraftInput{Slug: x.f.brandSlug + "-follows-default", BrandSlug: x.f.brandSlug, BrandName: "Fixture Brand",
		CategorySlug: x.f.category, CategoryName: "Fixture Category", Name: "Draft with shared shipping", Description: "Fixture",
		PriceCents: 1000, SKU: x.f.brandSlug + "-default", UseSellerParcelDefault: true}
	if _, err := catalog.CreateDraft(ctx, x.f.ownerID, input); !errors.Is(err, ports.ErrShippingInput) {
		t.Fatalf("missing seller default: %v", err)
	}
	var profileID int64
	if err := x.pool.QueryRow(ctx, `INSERT INTO seller_parcel_profiles(seller_id,name,weight_grams,length_cm,width_cm,height_cm)
		VALUES($1,'Fixture box',250,20,12,6) RETURNING id`, x.seller).Scan(&profileID); err != nil {
		t.Fatal(err)
	}
	if _, err := x.pool.Exec(ctx, `INSERT INTO seller_parcel_defaults(seller_id,profile_id) VALUES($1,$2)`, x.seller, profileID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = x.pool.Exec(context.Background(), `DELETE FROM seller_parcel_defaults WHERE seller_id=$1`, x.seller)
		_, _ = x.pool.Exec(context.Background(), `DELETE FROM seller_parcel_profiles WHERE seller_id=$1`, x.seller)
	})
	checkDraft := func(choice commerce.ProductDraftInput, weight int, inherited bool) {
		t.Helper()
		product, err := catalog.CreateDraft(ctx, x.f.ownerID, choice)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			_, _ = x.pool.Exec(context.Background(), `DELETE FROM inventory_stock WHERE variant_id=$1`, product.VariantID)
			_, _ = x.pool.Exec(context.Background(), `DELETE FROM products WHERE id=$1`, product.ID)
		})
		var gotWeight int
		var follows bool
		if err := x.pool.QueryRow(ctx, `SELECT p.shipping_weight_g,v.use_seller_parcel_default
			FROM marketplace_variant_parcels p JOIN product_variants v ON v.id=p.variant_id WHERE p.variant_id=$1`, product.VariantID).
			Scan(&gotWeight, &follows); err != nil || gotWeight != weight || follows != inherited {
			t.Fatalf("effective weight=%d default=%t error=%v", gotWeight, follows, err)
		}
	}
	checkDraft(input, 250, true)
	input.Slug, input.SKU = x.f.brandSlug+"-measured", x.f.brandSlug+"-measured"
	input.UseSellerParcelDefault = false
	input.ShippingParcel = &commerce.ParcelDimensions{WeightGrams: 740, LengthCM: 29, WidthCM: 18, HeightCM: 7}
	checkDraft(input, 740, false)
}

func TestSellerParcelProfilesResolveOverridesAndRejectStaleChanges(t *testing.T) {
	x := newMarketplaceFixture(t)
	ctx := t.Context()
	actor := x.f.ownerID
	variant := x.f.variantID
	if _, err := x.pool.Exec(ctx, `UPDATE product_variants SET shipping_weight_g=0, shipping_length_cm=0,
		shipping_width_cm=0, shipping_height_cm=0 WHERE id=$1`, variant); err != nil {
		t.Fatal(err)
	}
	first := ports.Parcel{WeightGrams: 300, LengthCM: 20, WidthCM: 12, HeightCM: 5}
	second := ports.Parcel{WeightGrams: 750, LengthCM: 30, WidthCM: 18, HeightCM: 8}
	if err := x.repo.CreateSellerParcel(ctx, actor, "Artwork box", first); err != nil {
		t.Fatal(err)
	}
	if err := x.repo.CreateSellerParcel(ctx, actor, "Large artwork box", second); err != nil {
		t.Fatal(err)
	}
	config, err := x.repo.SellerParcelSetup(ctx, actor, 0)
	if err != nil || len(config.Profiles) != 2 || len(config.Options) != 1 {
		t.Fatalf("setup: %+v, %v", config, err)
	}
	if _, err := x.repo.SellerParcelSetup(ctx, x.admin, 0); !errors.Is(err, ports.ErrForbidden) {
		t.Fatalf("seller ownership bypassed: %v", err)
	}
	other := newMarketplaceFixtureWithPool(t, x.pool)
	if _, err := x.repo.PreviewSellerParcelChange(ctx, other.f.ownerID, ports.SellerParcelChange{
		Action: "default", ProfileID: config.Profiles[0].ID,
	}); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("foreign profile accepted: %v", err)
	}
	apply := func(change ports.SellerParcelChange) ports.SellerParcelPreview {
		t.Helper()
		preview, err := x.repo.PreviewSellerParcelChange(ctx, actor, change)
		if err != nil {
			t.Fatal(err)
		}
		if err := x.repo.SaveSellerParcelChange(ctx, actor, change, preview.Token); err != nil {
			t.Fatal(err)
		}
		return preview
	}
	check := func(want ports.Parcel) {
		t.Helper()
		var got ports.Parcel
		err := x.pool.QueryRow(ctx, `SELECT shipping_weight_g,shipping_length_cm,shipping_width_cm,shipping_height_cm
			FROM marketplace_variant_readiness WHERE variant_id=$1`, variant).
			Scan(&got.WeightGrams, &got.LengthCM, &got.WidthCM, &got.HeightCM)
		if err != nil || got != want {
			t.Fatalf("effective parcel %+v, want %+v; %v", got, want, err)
		}
	}
	apply(ports.SellerParcelChange{Action: "default", ProfileID: config.Profiles[0].ID})
	check(ports.Parcel{})
	follow := ports.SellerParcelChange{Action: "assign", FollowDefault: true, OptionIDs: []int64{variant}}
	apply(follow)
	check(first)
	changeDefault := ports.SellerParcelChange{Action: "default", ProfileID: config.Profiles[1].ID}
	if preview := apply(changeDefault); len(preview.Options) != 1 || preview.Options[0].ID != variant {
		t.Fatalf("default preview missed affected variant: %+v", preview)
	}
	check(second)
	stale, err := x.repo.PreviewSellerParcelChange(ctx, actor, follow)
	if err != nil {
		t.Fatal(err)
	}
	if err := x.repo.SaveSetupParcel(ctx, x.admin, x.seller, variant, first); err != nil {
		t.Fatal(err)
	}
	if err := x.repo.SaveSellerParcelChange(ctx, actor, follow, stale.Token); !errors.Is(err, ports.ErrFulfillmentConflict) {
		t.Fatalf("stale preview changed a measured override: %v", err)
	}
	check(first)
}
