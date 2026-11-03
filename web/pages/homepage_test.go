package pages

import (
	"bytes"
	"context"
	"github.com/wecratfs/commerce/internal/domain/storefront"
	"github.com/wecratfs/commerce/internal/web/viewmodels"
	"strings"
	"testing"
)

func TestHomepageTrustRendersOnceBeforeFeatured(t *testing.T) {
	c := storefront.Defaults()
	for _, configured := range []bool{false, true} {
		p := viewmodels.CustomerPage{Route: "/", Title: "Home"}
		if configured {
			p.Storefront = &c
		}
		var out bytes.Buffer
		if err := Home(p).Render(context.Background(), &out); err != nil {
			t.Fatal(err)
		}
		html := out.String()
		if !strings.Contains(html, "Categories will appear here as collections are published.") {
			t.Fatal("empty category discovery must remain truthful")
		}
		trust := strings.Index(html, `class="trust-strip"`)
		featured := strings.Index(html, `Featured from our makers`)
		if strings.Count(html, `class="trust-strip"`) != 1 || trust < 0 || featured < trust {
			t.Fatal("trust must appear once before featured, including fallback Home")
		}
		if !strings.Contains(html, "homepage-reference.min.css") {
			t.Fatal("Home is missing its bounded presentation")
		}
	}
}

func TestOtherStorefrontPagesRetainHeaderAndFooter(t *testing.T) {
	c := storefront.Defaults()
	var out bytes.Buffer
	if err := Document(viewmodels.CustomerPage{Route: "/shipping", Title: "Shipping", Storefront: &c}).Render(context.Background(), &out); err != nil {
		t.Fatal(err)
	}
	html := out.String()
	if strings.Contains(html, "homepage-reference.min.css") || strings.Contains(html, `class="homepage-utility"`) {
		t.Fatal("homepage presentation leaked to another route")
	}
	if strings.Count(html, `class="trust-strip"`) != 1 {
		t.Fatal("other routes retain footer trust")
	}
}

func TestHomepageTrustWithoutCategorySection(t *testing.T) {
	sections := []storefront.Section{{ID: "hidden", Type: "categories", Hidden: true}, {ID: "featured", Type: "products", Order: 20}}
	if got := homepageTrustAnchor(sections); got != "featured" {
		t.Fatalf("hidden category cannot own visible trust: %q", got)
	}
}
