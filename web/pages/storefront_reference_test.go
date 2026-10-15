package pages

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/wecratfs/commerce/internal/web/viewmodels"
	"github.com/wecratfs/commerce/web/components"
)

func TestQuickViewRequiresARealOption(t *testing.T) {
	for _, test := range []struct {
		name     string
		variants []viewmodels.ProductVariant
		inStock  bool
		canAdd   bool
	}{
		{"unknown options", nil, true, false},
		{"technical default", []viewmodels.ProductVariant{{ID: "17", Name: "Default variant", InStock: true}}, true, true},
		{"named sole option", []viewmodels.ProductVariant{{ID: "17", Name: "Large", InStock: true}}, true, false},
		{"multiple options", []viewmodels.ProductVariant{{ID: "17", Name: "Small", InStock: true}, {ID: "18", Name: "Large"}}, true, false},
		{"sold out", []viewmodels.ProductVariant{{ID: "17", Name: "Default variant"}}, false, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			p := viewmodels.CustomerPage{Product: viewmodels.Product{Slug: "real-piece", Name: "Real piece", InStock: test.inStock, Variants: test.variants, CSRFToken: "test-csrf"}}
			var out bytes.Buffer
			if err := QuickView(p).Render(context.Background(), &out); err != nil {
				t.Fatal(err)
			}
			html := out.String()
			start := strings.Index(html, "<button")
			end := strings.Index(html[start:], ">") + start
			button := html[start:end]
			if enabled := !strings.Contains(button, " disabled"); enabled != test.canAdd {
				t.Fatalf("purchase enabled=%v, want %v: %s", enabled, test.canAdd, button)
			}
			if !strings.Contains(html, "/ui/cart/real-piece") || !strings.Contains(html, "/products/real-piece") {
				t.Fatal("quick view must use existing cart and product routes")
			}
			if strings.Contains(html, "<html") || strings.Contains(html, "site-header") {
				t.Fatal("quick view must be a fragment, not a nested document")
			}
			var card bytes.Buffer
			if err := components.ProductCard(p.Product, true).Render(context.Background(), &card); err != nil {
				t.Fatal(err)
			}
			if direct := strings.Contains(card.String(), `hx-post="/ui/cart/real-piece"`); direct != test.canAdd {
				t.Fatalf("card direct purchase=%v, want %v", direct, test.canAdd)
			}
			if showVariantChoice(p.Product) && !strings.Contains(html, "Select an option") {
				t.Fatal("real choices require explicit selection")
			}
		})
	}
}

func TestWishlistFragmentKeepsCardPresentation(t *testing.T) {
	product := viewmodels.Product{Slug: "real-piece", Name: "Real piece", InStock: true, Saved: true, Variants: []viewmodels.ProductVariant{{ID: "17"}}}
	for _, actions := range []bool{true, false} {
		var out bytes.Buffer
		if err := components.WishlistMutation(viewmodels.CustomerPage{}, product, false, actions).Render(context.Background(), &out); err != nil {
			t.Fatal(err)
		}
		html := out.String()
		if got := strings.Contains(html, "data-quick-view"); got != actions {
			t.Fatalf("quick view=%v, want %v", got, actions)
		}
		if got := strings.Contains(html, "product-card--compact"); got == actions {
			t.Fatalf("compact=%v, actions=%v", got, actions)
		}
		if !strings.Contains(html, "wishlist-count") {
			t.Fatal("wishlist counter must still update")
		}
	}
}
