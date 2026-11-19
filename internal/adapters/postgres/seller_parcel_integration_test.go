package postgres

import (
	"errors"
	"testing"

	"github.com/wecratfs/commerce/internal/ports"
)

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
