package pages

import (
	"bytes"
	"context"
	"github.com/wecratfs/commerce/internal/domain/storefront"
	"github.com/wecratfs/commerce/internal/web/viewmodels"
	"reflect"
	"strings"
	"testing"
)

func TestHomepageTrustRendersOnceAfterCollections(t *testing.T) {
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
		arrivals := strings.Index(html, `New arrivals`)
		collections := strings.Index(html, `Curated Collections`)
		if strings.Count(html, `class="trust-strip"`) != 1 || trust < collections || collections < featured || featured < arrivals || arrivals < 0 {
			t.Fatal("arrivals must precede featured, and trust must appear once after collections")
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
	if got := homepageTrustAnchor(sections); got != "" {
		t.Fatalf("hidden category cannot own visible trust: %q", got)
	}
}

func TestSavedHomepageOrderDoesNotMutateRelease(t *testing.T) {
	sections := []storefront.Section{{ID: "hero", Order: 10}, {ID: "featured", Type: "products", Order: 20}, {ID: "custom", Order: 30}, {ID: "arrivals", Type: "products", Order: 40}, {ID: "collections", Type: "collections", Order: 50}}
	before := append([]storefront.Section(nil), sections...)
	ordered := homepageSections(sections)
	var ids []string
	for _, section := range ordered {
		ids = append(ids, section.ID)
	}
	if !reflect.DeepEqual(ids, []string{"hero", "arrivals", "featured", "custom", "collections"}) || !reflect.DeepEqual(sections, before) {
		t.Fatal("presentation must reorder arrivals only and preserve saved release")
	}
}

func TestHomepageTrustWithHiddenOrMissingCollections(t *testing.T) {
	for _, hidden := range []bool{false, true} {
		c := storefront.Defaults()
		if hidden {
			c.Homepage[len(c.Homepage)-1].Hidden = true
		} else {
			c.Homepage = c.Homepage[:len(c.Homepage)-1]
		}
		var out bytes.Buffer
		if err := Home(viewmodels.CustomerPage{Route: "/", Title: "Home", Storefront: &c}).Render(context.Background(), &out); err != nil {
			t.Fatal(err)
		}
		html := out.String()
		if strings.Count(html, `class="trust-strip"`) != 1 || strings.Index(html, `class="trust-strip"`) < strings.Index(html, `heading-makers`) {
			t.Fatal("missing collections must put trust after homepage content")
		}
	}
}
