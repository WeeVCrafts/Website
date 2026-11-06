package pages

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/wecratfs/commerce/internal/web/viewmodels"
)

func TestGalleryNavigationGeometryIsKnownBeforeScripts(t *testing.T) {
	for _, multiple := range []bool{false, true} {
		product := viewmodels.Product{Name: "Handmade piece", Slug: "piece", ImageURL: "/cover.webp", Media: []viewmodels.ProductMedia{{URL: "/cover.webp"}}}
		if multiple {
			product.Media = append(product.Media, viewmodels.ProductMedia{URL: "/detail.webp"})
		}
		var out bytes.Buffer
		if err := ProductDetail(viewmodels.CustomerPage{Route: "/products/piece", Product: product}).Render(context.Background(), &out); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(out.String(), "data-gallery-single") == multiple {
			t.Fatal("server must reserve navigation only for multiple distinct media")
		}
	}
}
