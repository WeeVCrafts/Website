package postgres

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/wecratfs/commerce/internal/ports"
)

func TestFulfillmentSetupScopesSelectionsAndLoadsVerifiedProfiles(t *testing.T) {
	x := newMarketplaceFixture(t)
	ctx := context.Background()
	d, err := x.repo.FulfillmentSetup(ctx, x.admin, ports.MarketplaceSetupQuery{SellerID: x.seller, VariantID: x.f.variantID, SellerSearch: fmt.Sprint(x.seller)})
	if err != nil {
		t.Fatal(err)
	}
	if !d.InvoiceSaved || !d.TaxSaved || d.Invoice.SellerID != x.seller || d.HSN != "4420" || len(d.Variants) != 1 || d.Variants[0].ID != x.f.variantID {
		t.Fatalf("incorrect selected context: %+v", d)
	}
	if _, err = x.repo.FulfillmentSetup(ctx, x.f.ownerID, ports.MarketplaceSetupQuery{}); !errors.Is(err, ports.ErrForbidden) {
		t.Fatalf("seller saw private setup: %v", err)
	}
	other := newCommerceFixture(t, x.pool, 1, 5)
	if _, err = x.repo.FulfillmentSetup(ctx, x.admin, ports.MarketplaceSetupQuery{SellerID: x.seller, VariantID: other.variantID}); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("cross-seller variant: %v", err)
	}
	d, err = x.repo.FulfillmentSetup(ctx, x.admin, ports.MarketplaceSetupQuery{SellerID: x.seller, ProductSearch: "missing-fixture-product"})
	if err != nil || len(d.Variants) != 0 {
		t.Fatalf("no-results selector: %+v %v", d.Variants, err)
	}
}
