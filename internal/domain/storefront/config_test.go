package storefront

import (
	"testing"
)

func TestAcceptedDefaultsAndDeterministicOrdering(t *testing.T) {
	c := Defaults()
	if e := c.Validate(); e != nil {
		t.Fatal(e)
	}
	for _, v := range []struct{ amount, want int64 }{{0, 0}, {149999, 9900}, {150000, 0}, {150001, 0}} {
		if got := c.Settings.Policy.Shipping(v.amount); got != v.want {
			t.Fatalf("shipping(%d)=%d", v.amount, got)
		}
	}
	items := OrderedSections([]Section{{ID: "z", Order: 1}, {ID: "a", Order: 1}, {ID: "first", Order: 0}})
	if items[0].ID != "first" || items[1].ID != "a" {
		t.Fatal(items)
	}
}
func TestConfigurationRejectsExecutableAndInvalidValues(t *testing.T) {
	for _, mutate := range []func(*Config){func(c *Config) { c.Slides[0].Button.URL = "javascript:alert(1)" }, func(c *Config) { c.Theme.Primary = "red;display:none" }, func(c *Config) { c.Homepage = append(c.Homepage, c.Homepage[0]) }, func(c *Config) { c.Settings.Policy.PINRestricted = true }, func(c *Config) { c.Settings.Policy.PINCodes = []string{"000001"} }, func(c *Config) { c.Theme.BodyFont = "untrusted-font" }} {
		c := Defaults()
		mutate(&c)
		if c.Validate() == nil {
			t.Fatal("accepted invalid configuration")
		}
	}
	for _, v := range []string{"//other.example/", "/\\evil", "http://example.com", "data:image/svg+xml;base64,eA=="} {
		if SafeURL(v) {
			t.Fatalf("accepted URL %q", v)
		}
	}
}
func TestSpecificTemplateFallsBackAndPolicyServiceability(t *testing.T) {
	c := Defaults()
	c.Templates = append(c.Templates, Template{ID: "override", Kind: "product", Match: "product:wooden-box", Title: "Special"})
	if got := c.Template("product", "home", "wooden-box"); got.Title != "Special" {
		t.Fatal(got)
	}
	if got := c.Template("product", "home", "missing"); got.ID != "product" {
		t.Fatal(got)
	}
	p := Policy{PINRestricted: true, PINCodes: []string{"500001"}}
	if !p.Serves("500001") || p.Serves("411001") {
		t.Fatal("PIN restriction was ignored")
	}
}
