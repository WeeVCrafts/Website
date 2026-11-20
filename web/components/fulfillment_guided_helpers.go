package components

import (
	"fmt"
	"github.com/wecratfs/commerce/internal/ports"
	"github.com/wecratfs/commerce/internal/web/viewmodels"
)

func guidedStepURL(p viewmodels.MarketplacePage, key string) string {
	if key == "choose" {
		return p.OverviewURL()
	}
	return p.SetupURL("step", key, "pickup_choices", "")
}
func guidedCompleted(r ports.ShippingReadiness) string {
	n := 0
	for _, v := range []bool{r.InvoiceReady, r.PickupReady, r.ParcelReady(), r.TaxReady && r.Classified} {
		if v {
			n++
		}
	}
	return fmt.Sprint(n)
}
func guidedDetailsReady(r ports.ShippingReadiness) bool {
	return r.InvoiceReady && r.PickupReady && r.ParcelReady() && r.TaxReady && r.Classified
}
func guidedMeasurement(value int) string {
	if value <= 0 {
		return ""
	}
	return fmt.Sprint(value)
}

func guidedVerifiedLabel(ready bool) string {
	if ready {
		return "Verified"
	}
	return "Needs verification"
}

func guidedCatalogState(p viewmodels.MarketplacePage, r ports.ShippingReadiness) string {
	switch {
	case r.Stock <= 0:
		return "Needs available stock"
	case p.LiveOptionAllowed(r):
		return "Ready to buy"
	case p.Policy.ShippingMode == "pilot" && p.Policy.PilotVariantID == r.VariantID:
		return "Selected for pilot"
	case p.CheckoutEnabled && p.BookingsEnabled && p.ShippingConfigured && p.Policy.PublishedAt != nil:
		if p.Policy.PilotOrderID == 0 {
			return "Ready for pilot setup"
		}
		return "Ready for launch review"
	default:
		return "Needs launch review"
	}
}
