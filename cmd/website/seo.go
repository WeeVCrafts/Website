package main

import (
	"encoding/xml"
	"net/http"
	"net/url"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wecratfs/commerce/internal/identity"
)

type sitemapURLSet struct {
	XMLName xml.Name       `xml:"urlset"`
	XMLNS   string         `xml:"xmlns,attr"`
	URLs    []sitemapEntry `xml:"url"`
}
type sitemapEntry struct {
	Loc string `xml:"loc"`
}

func seoHandler(pool *pgxpool.Pool, publicURL string) (http.HandlerFunc, http.HandlerFunc) {
	base := strings.TrimRight(publicURL, "/")
	robots := func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("User-agent: *\nAllow: /\nDisallow: /account\nDisallow: /checkout\nDisallow: /orders\nDisallow: /cart\nDisallow: /wishlist\nDisallow: /login\nDisallow: /register\nDisallow: /admin\nDisallow: /seller-admin\nDisallow: /super-admin\nSitemap: " + base + "/sitemap.xml\n"))
	}
	sitemap := func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		set := sitemapURLSet{XMLNS: "http://www.sitemaps.org/schemas/sitemap/0.9"}
		for _, path := range []string{"/", "/products", "/categories", "/makers", "/brands", "/our-story", "/contact", "/shipping", "/faq"} {
			set.URLs = append(set.URLs, sitemapEntry{Loc: base + path})
		}
		if pool != nil {
			appendRows := func(query, prefix string, maker bool) error {
				rows, err := pool.Query(r.Context(), query)
				if err != nil {
					return err
				}
				defer rows.Close()
				for rows.Next() {
					var slug string
					if err := rows.Scan(&slug); err != nil {
						return err
					}
					if maker {
						slug = identity.MakerSlug(slug)
					}
					set.URLs = append(set.URLs, sitemapEntry{Loc: base + prefix + url.PathEscape(slug)})
				}
				return rows.Err()
			}
			for _, source := range sitemapCatalogSources {
				if err := appendRows(source.query, source.prefix, source.maker); err != nil {
					w.Header().Set("Retry-After", "300")
					http.Error(w, "Sitemap temporarily unavailable", http.StatusServiceUnavailable)
					return
				}
			}
		}
		w.Header().Set("Content-Type", "application/xml; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=300")
		if r.Method == http.MethodHead {
			return
		}
		_ = xml.NewEncoder(w).Encode(set)
	}
	return robots, sitemap
}

var sitemapCatalogSources = []struct {
	query, prefix string
	maker         bool
}{
	{`SELECT p.slug FROM products p JOIN sellers s ON s.id=p.seller_id WHERE p.status='approved' AND s.status='active' ORDER BY p.id LIMIT 5000`, "/products/", false},
	{`SELECT DISTINCT s.display_name FROM sellers s WHERE s.status='active' AND EXISTS (SELECT 1 FROM products p WHERE p.seller_id=s.id AND p.status='approved') ORDER BY s.display_name LIMIT 5000`, "/makers/", true},
	{`SELECT b.slug FROM brands b WHERE EXISTS (SELECT 1 FROM products p JOIN sellers s ON s.id=p.seller_id WHERE p.brand_slug=b.slug AND p.status='approved' AND s.status='active') ORDER BY b.slug LIMIT 5000`, "/brands/", false},
	{`SELECT c.slug FROM categories c WHERE EXISTS (SELECT 1 FROM product_categories pc JOIN products p ON p.id=pc.product_id JOIN sellers s ON s.id=p.seller_id WHERE pc.category_slug=c.slug AND p.status='approved' AND s.status='active') ORDER BY c.slug LIMIT 5000`, "/categories/", false},
}

func canonicalHostRedirect(next http.Handler, canonical string, environment string) http.Handler {
	canonicalURL, _ := url.Parse(strings.TrimRight(canonical, "/"))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if canonicalURL == nil || !strings.EqualFold(environment, "production") || r.Method != http.MethodGet && r.Method != http.MethodHead {
			next.ServeHTTP(w, r)
			return
		}
		host := strings.ToLower(strings.Split(r.Host, ":")[0])
		canonicalHost := strings.ToLower(canonicalURL.Hostname())
		if host != "" && host != canonicalHost && (strings.HasSuffix(host, ".up.railway.app") || strings.HasPrefix(host, "www.")) {
			location := *canonicalURL
			location.Path = r.URL.Path
			location.RawQuery = r.URL.RawQuery
			http.Redirect(w, r, location.String(), http.StatusPermanentRedirect)
			return
		}
		next.ServeHTTP(w, r)
	})
}
