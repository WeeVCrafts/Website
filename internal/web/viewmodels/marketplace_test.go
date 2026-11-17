package viewmodels

import (
	"github.com/wecratfs/commerce/internal/ports"
	"testing"
)

func TestGuidedSetupUsesFreshReadinessAndKeepsCheckoutSeparate(t *testing.T) {
	r := &ports.ShippingReadiness{PickupReady: true, WeightGrams: 500, LengthCM: 15, WidthCM: 12, HeightCM: 8}
	p := MarketplacePage{Setup: &ports.MarketplaceSetup{Query: ports.MarketplaceSetupQuery{VariantID: 1}, Selected: r}}
	if p.FirstMissingStep() != "invoice" || p.StepComplete("review") {
		t.Fatal("incomplete profile incorrectly passed")
	}
	r.InvoiceReady = true
	if p.FirstMissingStep() != "tax" {
		t.Fatal("did not skip saved pickup and parcel")
	}
	r.TaxReady = true
	r.Classified = true
	if p.FirstMissingStep() != "review" || !p.StepComplete("review") {
		t.Fatal("saved details were not reflected")
	}
	if p.PurchaseReady() {
		t.Fatal("completed setup enabled checkout without stock, policy or switches")
	}
	r.Stock = 1
	r.Selected = true
	p.CheckoutEnabled = true
	p.BookingsEnabled = true
	if p.PurchaseReady() {
		t.Fatal("unpublished policy enabled checkout")
	}
}
