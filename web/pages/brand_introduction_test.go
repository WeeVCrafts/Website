package pages

import (
	"bytes"
	"strings"
	"testing"

	"github.com/wecratfs/commerce/internal/domain/storefront"
	"github.com/wecratfs/commerce/internal/web/viewmodels"
)

func TestHomepageIntroductionFollowsEitherHeroRendererOnce(t *testing.T) {
	config := storefront.Defaults()
	for _, published := range []bool{false, true} {
		page := viewmodels.CustomerPage{Route: "/", PublicURL: "https://weevcrafts.com"}
		if published {
			page.Storefront = &config
		}
		var output bytes.Buffer
		if err := Home(page).Render(t.Context(), &output); err != nil {
			t.Fatal(err)
		}
		html := output.String()
		intro := strings.Index(html, `class="homepage-introduction"`)
		hero := strings.Index(html, `data-hero`)
		categories := strings.Index(html, `data-category-rail`)
		if intro < 0 || hero < 0 || categories < intro || intro < hero || strings.Count(html, `id="homepage-introduction-title"`) != 1 {
			t.Fatalf("published=%v: introduction placement/count is invalid", published)
		}
		if !strings.Contains(html, "Discover Weevcrafts — Handmade Crafts from India") || !strings.Contains(html, "independent Indian makers.") {
			t.Fatal("requested introduction is missing")
		}
	}
}
func TestEditorialBrandNormalizationRetainsStoredTextAndLinkTargets(t *testing.T) {
	legacy := "Welcome to **WeeVCrafts**, [WeevCrafts story](/pages/WeevCrafts-story)."
	spans := storefrontTextSpans(legacy)
	if legacy != "Welcome to **WeeVCrafts**, [WeevCrafts story](/pages/WeevCrafts-story)." {
		t.Fatal("saved editorial input changed")
	}
	foundLink := false
	for _, span := range spans {
		if strings.Contains(span.Text, "WeeVCrafts") || strings.Contains(span.Text, "WeevCrafts") {
			t.Fatal("visible editorial brand was not normalized")
		}
		if span.Kind == "link" {
			foundLink = true
			if span.URL != "/pages/WeevCrafts-story" || span.Text != "Weevcrafts story" {
				t.Fatalf("link target or display label changed incorrectly: %+v", span)
			}
		}
	}
	if !foundLink {
		t.Fatal("link was lost")
	}
	page := viewmodels.CustomerPage{Route: "/our-story", Title: "Our Story — WeeVCrafts", Description: "Made by WeevCrafts makers"}
	if strings.Contains(pageDocumentTitle(page), "WeeVCrafts") || strings.Contains(pageDescription(page), "WeevCrafts") {
		t.Fatal("metadata retained legacy brand")
	}
}
