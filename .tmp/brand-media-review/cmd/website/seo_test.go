package main

import (
	"context"
	"encoding/xml"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wecratfs/commerce/internal/adapters/postgres"
	"github.com/wecratfs/commerce/internal/domain/storefront"
	"github.com/wecratfs/commerce/internal/identity"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
)

func TestSitemapProtocolNamespace(t *testing.T) {
	_, sitemap := seoHandler(nil, "https://weevcrafts.com")
	w := httptest.NewRecorder()
	sitemap(w, httptest.NewRequest("GET", "/sitemap.xml", nil))
	var document struct {
		XMLName xml.Name
		URLs    []struct {
			Loc string `xml:"loc"`
		} `xml:"url"`
	}
	if err := xml.Unmarshal(w.Body.Bytes(), &document); err != nil {
		t.Fatal(err)
	}
	if document.XMLName.Space != "http://www.sitemaps.org/schemas/sitemap/0.9" || document.XMLName.Local != "urlset" {
		t.Fatalf("invalid sitemap namespace: %v", document.XMLName)
	}
	if len(document.URLs) == 0 || document.URLs[0].Loc != "https://weevcrafts.com/" {
		t.Fatal("missing canonical homepage")
	}
}

func TestSitemapCatalogQueriesAgainstPostgres(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL required for catalog sitemap regression")
	}
	pool, err := pgxpool.New(t.Context(), databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := postgres.ApplyMigrations(t.Context(), pool); err != nil {
		t.Fatal(err)
	}
	_, sitemap := seoHandler(pool, "https://weevcrafts.com")
	w := httptest.NewRecorder()
	sitemap(w, httptest.NewRequest(http.MethodGet, "/sitemap.xml", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("catalog query failed: %d %s", w.Code, w.Body.String())
	}
	var document sitemapURLSet
	if err := xml.Unmarshal(w.Body.Bytes(), &document); err != nil {
		t.Fatal(err)
	}
	rows, err := pool.Query(t.Context(), `SELECT DISTINCT s.display_name FROM sellers s WHERE s.status='active' AND EXISTS (SELECT 1 FROM products p WHERE p.seller_id=s.id AND p.status='approved') ORDER BY s.display_name LIMIT 5000`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	urls := make(map[string]bool)
	for _, entry := range document.URLs {
		urls[entry.Loc] = true
	}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		want := "https://weevcrafts.com/makers/" + url.PathEscape(identity.MakerSlug(name))
		if !urls[want] {
			t.Errorf("public maker missing from sitemap: %s", want)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
}

func TestSitemapDatabaseFailureDoesNotPublishIncompleteCatalog(t *testing.T) {
	pool, err := pgxpool.New(context.Background(), "postgres://unused:unused@127.0.0.1:1/unused")
	if err != nil {
		t.Fatal(err)
	}
	pool.Close()
	_, sitemap := seoHandler(pool, "https://weevcrafts.com")
	w := httptest.NewRecorder()
	sitemap(w, httptest.NewRequest(http.MethodGet, "/sitemap.xml", nil))
	if w.Code != http.StatusServiceUnavailable || w.Header().Get("Retry-After") != "300" || strings.Contains(w.Body.String(), "<urlset") {
		t.Fatalf("database failure must be retryable, not a cached partial sitemap: %d %s", w.Code, w.Body.String())
	}
}

func TestCanonicalRedirectPreservesDiscoveryPathAndQuery(t *testing.T) {
	called := false
	h := canonicalHostRedirect(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true; w.WriteHeader(204) }), "https://weevcrafts.com", "production")
	for _, host := range []string{"www.weevcrafts.com", "website-example.up.railway.app"} {
		r := httptest.NewRequest(http.MethodGet, "https://"+host+"/products/blue-bowl?option=2", nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != http.StatusPermanentRedirect || w.Header().Get("Location") != "https://weevcrafts.com/products/blue-bowl?option=2" || called {
			t.Fatalf("incorrect canonical redirect: %d %s", w.Code, w.Header().Get("Location"))
		}
	}
	r := httptest.NewRequest(http.MethodPost, "https://www.weevcrafts.com/api/v1/checkout", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if !called || w.Code != 204 {
		t.Fatal("canonical redirect must not redirect checkout mutations")
	}
}

func TestSitemapIncludesOnlyVisibleCanonicalContentPages(t *testing.T) {
	config := storefront.Config{Pages: []storefront.Page{{Slug: "returns"}, {Slug: "shipping", Legacy: true}, {Slug: "hidden", Hidden: true}}}
	paths := sitemapContentPaths(config)
	joined := strings.Join(paths, " ")
	if !strings.Contains(joined, "/pages/returns") || !strings.Contains(joined, "/shipping") || strings.Contains(joined, "/pages/shipping") || strings.Contains(joined, "hidden") {
		t.Fatalf("sitemap content paths = %v", paths)
	}
}

func TestLegacyStorefrontRedirectsHaveRelevantReplacements(t *testing.T) {
	h := canonicalHostRedirect(http.NotFoundHandler(), "https://weevcrafts.com", "production")
	for _, tc := range []struct{ source, target string }{{"/pages/about-us", "/about"}, {"/collections/frontpage", "/products"}, {"/collections/arts-collection", "/categories/arts"}, {"/collections/sarees", "/categories/textiles"}} {
		r := httptest.NewRequest(http.MethodGet, "https://weevcrafts.com"+tc.source, nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != http.StatusMovedPermanently || w.Header().Get("Location") != "https://weevcrafts.com"+tc.target {
			t.Fatalf("redirect %s: %d %s", tc.source, w.Code, w.Header().Get("Location"))
		}
	}
	for _, path := range []string{"/collections/handmade-bangles", "/collections/home-made-pickles", "/collections/warli-painting", "/products/saree", "/not-a-real-page"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != http.StatusNotFound {
			t.Fatalf("unmatched content %s must retain its real 404", path)
		}
	}
}
