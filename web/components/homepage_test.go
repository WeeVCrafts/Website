package components

import (
	"github.com/wecratfs/commerce/internal/domain/storefront"
	"github.com/wecratfs/commerce/internal/web/viewmodels"
	"strings"
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

func TestStorefrontLogoUsesImmutableAssetsWithoutChangingCustomMedia(t *testing.T) {
	logo := storefrontLogo(viewmodels.CustomerPage{})
	if !strings.HasPrefix(logo, "/assets/images/brand/weevcrafts-logo.png?v=") {
		t.Fatal("maintained logo must use content-versioned delivery")
	}
	for _, source := range []string{"/media/storefront/41", "https://example.com/logo.png", "/assets/images/custom.png?signature=retained"} {
		c := storefront.Defaults()
		c.Theme.Logo = source
		if got := storefrontLogo(viewmodels.CustomerPage{Storefront: &c}); got != source {
			t.Fatalf("custom logo URL changed: %q", got)
		}
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
