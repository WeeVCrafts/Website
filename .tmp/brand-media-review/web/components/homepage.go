package components

import (
	"fmt"
	"github.com/a-h/templ"
	"github.com/wecratfs/commerce/internal/web/viewmodels"
)

func homepageShippingText(p viewmodels.CustomerPage) string {
	if p.Storefront != nil && p.Storefront.Settings.Policy.FreeShippingCents > 0 {
		cents := p.Storefront.Settings.Policy.FreeShippingCents
		amount := fmt.Sprintf("%d", cents/100)
		if cents%100 != 0 {
			amount = fmt.Sprintf("%d.%02d", cents/100, cents%100)
		}
		return "Free shipping on orders of ₹" + amount + "+"
	}
	return "Delivery to serviceable PIN codes in India"
}

// Home uses a document navigation so its bounded header variant cannot survive
// an HTMX content-only transition to a different shopping route.
func headerSearchAttributes(p viewmodels.CustomerPage) templ.Attributes {
	if p.Route == "/" {
		return nil
	}
	return templ.Attributes{"hx-get": "/search", "hx-select": "#page", "hx-target": "#page", "hx-swap": "outerHTML", "hx-push-url": "true", "hx-trigger": "submit", "hx-sync": "#page:replace", "hx-indicator": "#global-search-indicator"}
}
