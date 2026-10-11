package pages

import (
	"bytes"
	"encoding/json"
	"regexp"
	"strings"
	"testing"

	"github.com/wecratfs/commerce/internal/web/viewmodels"
)

func TestCustomerPageSEOIndexingPolicy(t *testing.T) {
	for _, test := range []struct {
		route string
		want  string
	}{
		{route: "/", want: "index,follow"},
		{route: "/products/blue-bowl", want: "index,follow"},
		{route: "/account", want: "noindex,nofollow"},
		{route: "/faq", want: "index,follow"},
	} {
		if got := pageRobots(viewmodels.CustomerPage{Route: test.route}); got != test.want {
			t.Fatalf("pageRobots(%q) = %q, want %q", test.route, got, test.want)
		}
	}
}

func TestProductStructuredDataIsValidAndComplete(t *testing.T) {
	page := viewmodels.CustomerPage{
		Route:     "/products/blue-bowl",
		PublicURL: "https://weevcrafts.example",
		Title:     "Blue Bowl",
		Description: "Handmade blue bowl",
		Product: viewmodels.Product{
			Name: "Blue Bowl", Description: "Handmade blue bowl", ImageURL: "/assets/bowl.webp",
			SKU: "BOWL-1", PriceCents: 1250, Available: 3, InStock: true,
			Seller: "Clay Studio", Rating: "4.8", Reviews: 12,
		},
	}
	var data map[string]any
	if err := json.Unmarshal([]byte(productStructuredData(page)), &data); err != nil {
		t.Fatalf("product JSON-LD is invalid: %v", err)
	}
	if data["@type"] != "Product" || data["name"] != "Blue Bowl" {
		t.Fatalf("product JSON-LD identity = %#v", data)
	}
	offers, ok := data["offers"].(map[string]any)
	if !ok || offers["price"] != "12.50" || offers["availability"] != "https://schema.org/InStock" {
		t.Fatalf("product JSON-LD offers = %#v", data["offers"])
	}
	if !strings.Contains(pageOGImage(page), "https://weevcrafts.example/assets/bowl.webp") {
		t.Fatalf("product OG image = %q", pageOGImage(page))
	}
}

func TestDocumentRendersEscapedSEOMetadata(t *testing.T) {
	page := viewmodels.CustomerPage{
		Route: "/products/safe",
		Title: "A & B",
		Description: `A < handmade product`,
		Product: viewmodels.Product{Name: "A & B", Description: `A < handmade product`, ImageURL: "/safe.webp", PriceCents: 100},
	}
	var body bytes.Buffer
	if err := Document(page).Render(t.Context(), &body); err != nil {
		t.Fatalf("render document: %v", err)
	}
	output := body.String()
	script := regexp.MustCompile(`<script[^>]*type="application/ld\+json"[^>]*>([\s\S]*?)</script>`).FindStringSubmatch(output)
	if len(script) != 2 {
		t.Fatal("rendered product schema is missing")
	}
	var schema map[string]any
	if err := json.Unmarshal([]byte(script[1]), &schema); err != nil {
		t.Fatalf("rendered product schema must contain JSON: %v", err)
	}
	if schema["name"] != "A & B" || schema["@type"] != "Product" {
		t.Fatalf("rendered product schema = %#v", schema)
	}
	for _, want := range []string{`name="robots" content="index,follow"`, `rel="canonical"`, `property="og:type" content="product"`, `application/ld+json`, `&amp;`, `&lt;`} {
		if !strings.Contains(output, want) {
			t.Fatalf("document missing %q: %s", want, output)
		}
	}
}
