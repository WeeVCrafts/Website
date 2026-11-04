package components

import (
	"github.com/wecratfs/commerce/internal/domain/storefront"
	"github.com/wecratfs/commerce/internal/web/viewmodels"
	"testing"
)

func TestHomepageShippingUsesPublishedPolicy(t *testing.T) {
	c := storefront.Defaults()
	p := viewmodels.CustomerPage{Storefront: &c}
	for _, tc := range []struct {
		cents int64
		want  string
	}{{150000, "Free shipping on orders of ₹1500+"}, {99950, "Free shipping on orders of ₹999.50+"}, {0, "Delivery to serviceable PIN codes in India"}} {
		c.Settings.Policy.FreeShippingCents = tc.cents
		if got := homepageShippingText(p); got != tc.want {
			t.Fatalf("shipping: %q, want %q", got, tc.want)
		}
	}
	if got := homepageShippingText(viewmodels.CustomerPage{}); got != "Delivery to serviceable PIN codes in India" {
		t.Fatal(got)
	}
}

func TestHomepageSearchUsesDocumentNavigation(t *testing.T) {
	if len(headerSearchAttributes(viewmodels.CustomerPage{Route: "/"})) != 0 {
		t.Fatal("homepage search must navigate the document")
	}
	if headerSearchAttributes(viewmodels.CustomerPage{Route: "/products"})["hx-target"] != "#page" {
		t.Fatal("other storefront searches retain shared fragment navigation")
	}
}
