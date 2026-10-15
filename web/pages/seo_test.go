package pages

import (
	"bytes"
	"encoding/json"
	"regexp"
	"strings"
	"testing"

	"github.com/wecratfs/commerce/internal/identity"
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
		{route: "/checkout", want: "noindex,nofollow"},
		{route: "/admin", want: "noindex,nofollow"},
		{route: "/faq", want: "index,follow"},
	} {
		if got := pageRobots(viewmodels.CustomerPage{Route: test.route}); got != test.want {
			t.Fatalf("pageRobots(%q) = %q, want %q", test.route, got, test.want)
		}
	}
}

func TestHomepageBrandMetadataIsConsistent(t *testing.T) {
	page := viewmodels.CustomerPage{Route: "/", Title: "Handmade Today", PublicURL: "https://weevcrafts.com", Description: "Old description"}
	var body bytes.Buffer
	if err := Document(page).Render(t.Context(), &body); err != nil {
		t.Fatal(err)
	}
	html := body.String()
	for _, want := range []string{
		`<title>WeeVCrafts | Handmade Indian Arts, Crafts &amp; Sarees</title>`,
		`property="og:title" content="WeeVCrafts | Handmade Indian Arts, Crafts &amp; Sarees"`,
		`property="og:site_name" content="WeeVCrafts"`,
		`name="description" content="` + identity.Description + `"`,
		`rel="canonical" href="https://weevcrafts.com/"`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("missing brand metadata: %s", want)
		}
	}
	if strings.Count(html, `id="website-schema"`) != 1 {
		t.Fatal("homepage must render exactly one website schema block")
	}
	var data struct {
		Graph []struct {
			Type          string   `json:"@type"`
			Name          string   `json:"name"`
			URL           string   `json:"url"`
			Logo          string   `json:"logo"`
			AlternateName []string `json:"alternateName"`
		} `json:"@graph"`
	}
	if err := json.Unmarshal([]byte(websiteStructuredData(page)), &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Graph) != 2 || data.Graph[0].Type != "WebSite" || data.Graph[1].Type != "Organization" {
		t.Fatalf("expected one website and one organization: %+v", data.Graph)
	}
	for _, node := range data.Graph {
		if node.Name != identity.Name || node.URL != "https://weevcrafts.com/" {
			t.Errorf("inconsistent brand identity: %+v", node)
		}
	}
	if len(data.Graph[0].AlternateName) == 0 || data.Graph[1].Logo != "https://weevcrafts.com/assets/images/brand/weevcrafts-logo.png" {
		t.Fatal("brand aliases and supplied logo must be preserved")
	}
	if websiteStructuredData(viewmodels.CustomerPage{Route: "/login"}) != "" {
		t.Fatal("website identity schema belongs on the homepage")
	}
}

func TestPageTitleDoesNotDuplicateBrand(t *testing.T) {
	for _, title := range []string{"Account · WeeVCrafts", "Sign in - WeeVCrafts"} {
		if got := pageDocumentTitle(viewmodels.CustomerPage{Route: "/login", Title: title}); got != title {
			t.Errorf("title = %q, want %q", got, title)
		}
	}
}

func TestProductStructuredDataIsValidAndComplete(t *testing.T) {
	page := viewmodels.CustomerPage{
		Route:       "/products/blue-bowl",
		PublicURL:   "https://weevcrafts.example",
		Title:       "Blue Bowl",
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
		Route:       "/products/safe",
		Title:       "A & B",
		Description: `A < handmade product`,
		Product:     viewmodels.Product{Name: "A & B", Description: `A < handmade product`, ImageURL: "/safe.webp", PriceCents: 100},
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
