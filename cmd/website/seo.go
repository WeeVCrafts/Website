package main

import (
	"encoding/xml"
	"net/http"
	"net/url"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

type sitemapURLSet struct {
	XMLName xml.Name       `xml:"urlset"`
	XMLNS   string         `xml:"xmlns,attr"`
	URLs    []sitemapEntry `xml:"url"`
}
type sitemapEntry struct { Loc string `xml:"loc"` }

func seoHandler(pool *pgxpool.Pool, publicURL string) (http.HandlerFunc, http.HandlerFunc) {
	base := strings.TrimRight(publicURL, "/")
	robots := func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead { w.WriteHeader(http.StatusMethodNotAllowed); return }
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("User-agent: *\nAllow: /\nDisallow: /account\nDisallow: /checkout\nDisallow: /orders\nDisallow: /cart\nDisallow: /wishlist\nDisallow: /login\nDisallow: /register\nDisallow: /admin\nDisallow: /seller-admin\nDisallow: /super-admin\nSitemap: " + base + "/sitemap.xml\n"))
	}
	sitemap := func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead { w.WriteHeader(http.StatusMethodNotAllowed); return }
		set := sitemapURLSet{XMLNS: "https://www.sitemaps.org/schemas/sitemap/0.9"}
		for _, path := range []string{"/", "/products", "/categories", "/makers", "/brands", "/our-story", "/contact", "/shipping", "/faq"} { set.URLs = append(set.URLs, sitemapEntry{Loc: base + path}) }
		if pool != nil {
			appendRows := func(query, prefix string) {
				rows, err := pool.Query(r.Context(), query); if err != nil { return }; defer rows.Close()
				for rows.Next() { var slug string; if rows.Scan(&slug) == nil { set.URLs = append(set.URLs, sitemapEntry{Loc: base + prefix + url.PathEscape(slug)}) } }
			}
			appendRows("SELECT slug FROM products WHERE status='approved' ORDER BY id LIMIT 5000", "/products/")
			appendRows("SELECT slug FROM sellers WHERE status='active' ORDER BY id LIMIT 5000", "/makers/")
			appendRows("SELECT slug FROM brands WHERE EXISTS (SELECT 1 FROM products p WHERE p.brand_slug=brands.slug AND p.status='approved') ORDER BY slug LIMIT 5000", "/brands/")
			appendRows("SELECT slug FROM categories WHERE EXISTS (SELECT 1 FROM products p WHERE p.category_slug=categories.slug AND p.status='approved') ORDER BY slug LIMIT 5000", "/products?category=")
		}
		w.Header().Set("Content-Type", "application/xml; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=300")
		if r.Method == http.MethodHead { return }
		_ = xml.NewEncoder(w).Encode(set)
	}
	return robots, sitemap
}

func canonicalHostRedirect(next http.Handler, canonical string, environment string) http.Handler {
	canonicalURL, _ := url.Parse(strings.TrimRight(canonical, "/"))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if canonicalURL == nil || !strings.EqualFold(environment, "production") || r.Method != http.MethodGet && r.Method != http.MethodHead { next.ServeHTTP(w, r); return }
		host := strings.ToLower(strings.Split(r.Host, ":")[0])
		canonicalHost := strings.ToLower(canonicalURL.Hostname())
		if host != "" && host != canonicalHost && (strings.HasSuffix(host, ".up.railway.app") || strings.HasPrefix(host, "www.")) {
			location := *canonicalURL; location.Path = r.URL.Path; location.RawQuery = r.URL.RawQuery
			http.Redirect(w, r, location.String(), http.StatusPermanentRedirect); return
		}
		next.ServeHTTP(w, r)
	})
}
