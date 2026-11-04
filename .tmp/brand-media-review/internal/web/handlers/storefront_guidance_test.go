package handlers

import (
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/wecratfs/commerce/internal/domain/storefront"
	"github.com/wecratfs/commerce/internal/web/viewmodels"
)

func TestSlideActionsPreserveContentAndHiddenSlides(t *testing.T) {
	m := viewmodels.StorefrontEditor{Tab: "hero", Selected: "two"}
	m.Draft.Config = storefront.Defaults()
	m.Draft.Config.Slides = []storefront.Slide{{ID: "one", Order: 10, Title: "First", Image: "/one.jpg"}, {ID: "two", Order: 20, Title: "Second", Image: "/two.jpg", Hidden: true}}
	for _, operation := range []string{"up", "hide"} {
		r := httptest.NewRequest("POST", "/", strings.NewReader(url.Values{"operation": {operation}}.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		_ = r.ParseForm()
		if err := applyEditorForm(r, &m); err != nil {
			t.Fatal(err)
		}
	}
	if m.Draft.Config.Slides[0].ID != "two" || m.Draft.Config.Slides[0].Hidden || m.Draft.Config.Slides[0].Image != "/two.jpg" || m.Draft.Config.Slides[1].Title != "First" {
		t.Fatalf("slide action lost content: %+v", m.Draft.Config.Slides)
	}
}

func TestCategoryCreationIncludesUploadedImageAndPresentation(t *testing.T) {
	m := viewmodels.StorefrontEditor{Tab: "categories"}
	m.Draft.Config = storefront.Defaults()
	r := httptest.NewRequest("POST", "/", strings.NewReader(url.Values{"operation": {"add"}, "name": {"Studio baskets"}, "slug": {"studio-baskets"}, "image": {"/assets/images/customer/ceramic-mugs.webp"}, "text": {"Handwoven pieces"}, "order": {"75"}, "visible": {"true"}}.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	_ = r.ParseForm()
	if err := applyEditorForm(r, &m); err != nil {
		t.Fatal(err)
	}
	c := m.Draft.Config.Categories[len(m.Draft.Config.Categories)-1]
	if c.Image == "" || c.Order != 75 || c.Description != "Handwoven pieces" || !c.Visible || m.Selected != c.Slug {
		t.Fatalf("category presentation lost: %+v", c)
	}
}

func TestStructuredNavigationPreservesPlainTextAndRejectsUnsafeDestinations(t *testing.T) {
	for _, tc := range []struct {
		rows    string
		invalid bool
	}{{`[{"label":"Arts | Crafts","url":"/products?q=craft"}]`, false}, {`[{"label":"Unsafe","url":"javascript:alert(1)"}]`, true}, {`not-json`, true}} {
		m := viewmodels.StorefrontEditor{Tab: "navigation"}
		m.Draft.Config = storefront.Defaults()
		r := httptest.NewRequest("POST", "/", strings.NewReader(url.Values{"navigation_rows": {tc.rows}, "trust": {""}, "policy_links": {""}, "footer": {""}}.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		_ = r.ParseForm()
		err := applyEditorForm(r, &m)
		if (err != nil) != tc.invalid {
			t.Fatalf("rows %s error %v", tc.rows, err)
		}
		if !tc.invalid && m.Draft.Config.Navigation[0].Label != "Arts | Crafts" {
			t.Fatal("structured text was split by legacy separators")
		}
	}
}

func TestSectionPresetsDoNotInventProductsOrMakerIdentities(t *testing.T) {
	for _, kind := range []string{"curated", "gifts", "story", "promotion", "questions"} {
		s := storefrontPreset(kind, 10)
		c := storefront.Defaults()
		c.Homepage = append(c.Homepage, s)
		if err := c.Validate(); err != nil {
			t.Fatalf("%s: %v", kind, err)
		}
		if len(s.Selection.Products) != 0 || len(s.Items) != 0 {
			t.Fatalf("%s invented content", kind)
		}
	}
	m := viewmodels.StorefrontEditor{}
	m.Draft.Config = storefront.Defaults()
	m.Draft.Config.Homepage = []storefront.Section{storefrontPreset("curated", 10)}
	storefrontGuidance(&m)
	if len(m.Readiness) == 0 || !m.Readiness[0].Warning {
		t.Fatal("empty selected collection should warn, not fabricate items")
	}
}
