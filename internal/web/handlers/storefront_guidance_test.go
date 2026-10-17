package handlers

import (
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/wecratfs/commerce/internal/domain/storefront"
	"github.com/wecratfs/commerce/internal/web/viewmodels"
)

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
