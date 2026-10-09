package pages

import (
	"bytes"
	"net/url"
	"strings"
	"testing"

	"github.com/wecratfs/commerce/internal/web/viewmodels"
)

func TestCategoryCanBeClearedFromItsListingURL(t *testing.T) {
	page := viewmodels.CustomerPage{Route: "/products", Category: "wellness", Filters: url.Values{"category": {"wellness"}}, Categories: []viewmodels.Category{{Slug: "wellness", Name: "Wellness"}}}
	var body bytes.Buffer
	if err := Listing(page).Render(t.Context(), &body); err != nil { t.Fatal(err) }
	if strings.Contains(body.String(), `type="hidden" name="category"`) {
		t.Fatal("hidden category keeps filter selected after its checkbox is cleared")
	}
}
