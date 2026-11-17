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
