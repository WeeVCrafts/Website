package main

import (
	"encoding/xml"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wecratfs/commerce/internal/adapters/postgres"
	"github.com/wecratfs/commerce/internal/domain/storefront"
	"github.com/wecratfs/commerce/internal/identity"
)

type sitemapURLSet struct {
	XMLName    xml.Name       `xml:"urlset"`
	XMLNS      string         `xml:"xmlns,attr"`
	ImageXMLNS string         `xml:"xmlns:image,attr,omitempty"`
	URLs       []sitemapEntry `xml:"url"`
}
type sitemapEntry struct {
	Loc        string        `xml:"loc"`
	LastMod    string        `xml:"lastmod,omitempty"`
	ChangeFreq string        `xml:"changefreq,omitempty"`
	Priority   string        `xml:"priority,omitempty"`
	Image      *sitemapImage `xml:"image:image,omitempty"`
}
type sitemapImage struct {
	Loc   string `xml:"image:loc"`
	Title string `xml:"image:title,omitempty"`
}

func seoHandler(pool *pgxpool.Pool, publicURL string) (http.HandlerFunc, http.HandlerFunc) {
	base := strings.TrimRight(publicURL, "/")
	robots := func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=86400")
		_, _ = w.Write([]byte("User-agent: *\nAllow: /\nDisallow: /account\nDisallow: /checkout\nDisallow: /orders\nDisallow: /cart\nDisallow: /wishlist\nDisallow: /login\nDisallow: /register\nDisallow: /admin\nDisallow: /seller-admin\nDisallow: /super-admin\n\nCrawl-delay: 1\n\nSitemap: " + base + "/sitemap.xml\n"))
	}
	sitemap := func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		set := sitemapURLSet{
			XMLNS:      "http://www.sitemaps.org/schemas/sitemap/0.9",
			ImageXMLNS: "http://www.google.com/schemas/sitemap-image/1.1",
		}
		content := storefront.Defaults()
		if pool != nil {
			revision, err := postgres.ReadPublishedStorefront(r.Context(), pool)
			if err != nil {
				w.Header().Set("Retry-After", "300")
				http.Error(w, "Sitemap temporarily unavailable", http.StatusServiceUnavailable)
				return
			}
			content = revision.Config
		}
		for _, path := range sitemapContentPaths(content) {
			priority := "0.8"
			changeFreq := "weekly"
			if path == "/" {
				priority = "1.0"
				changeFreq = "daily"
			} else if path == "/categories" || path == "/products" {
				priority = "0.9"
				changeFreq = "daily"
			}
			set.URLs = append(set.URLs, sitemapEntry{
				Loc:        base + path,
				Priority:   priority,
				ChangeFreq: changeFreq,
			})
		}
		if pool != nil {
			// Query products with rich images and modification dates
			prodRows, err := pool.Query(r.Context(), `
				SELECT p.slug, p.name, COALESCE(p.image_url, ''), p.updated_at
				FROM products p
				JOIN sellers s ON s.id=p.seller_id
				WHERE p.status='approved' AND s.status='active'
				ORDER BY p.id LIMIT 5000`)
			if err != nil {
				w.Header().Set("Retry-After", "300")
				http.Error(w, "Sitemap temporarily unavailable", http.StatusServiceUnavailable)
				return
			}
			defer prodRows.Close()
			for prodRows.Next() {
				var slug, name, imageURL string
				var updatedAt time.Time
				if err := prodRows.Scan(&slug, &name, &imageURL, &updatedAt); err != nil {
					w.Header().Set("Retry-After", "300")
					http.Error(w, "Sitemap temporarily unavailable", http.StatusServiceUnavailable)
					return
				}
				entry := sitemapEntry{
					Loc:        base + "/products/" + url.PathEscape(slug),
					LastMod:    updatedAt.UTC().Format("2006-01-02"),
					ChangeFreq: "weekly",
					Priority:   "0.8",
				}
				if imageURL != "" {
					imgLoc := imageURL
					if strings.HasPrefix(imgLoc, "/") {
						imgLoc = base + imgLoc
					}
					entry.Image = &sitemapImage{
						Loc:   imgLoc,
						Title: name,
					}
				}
				set.URLs = append(set.URLs, entry)
			}
			if err := prodRows.Err(); err != nil {
				w.Header().Set("Retry-After", "300")
				http.Error(w, "Sitemap temporarily unavailable", http.StatusServiceUnavailable)
				return
			}

			appendRows := func(query, prefix string, maker bool, priority, changeFreq string) error {
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
					set.URLs = append(set.URLs, sitemapEntry{
						Loc:        base + prefix + url.PathEscape(slug),
						Priority:   priority,
						ChangeFreq: changeFreq,
					})
				}
				return rows.Err()
			}
			for _, source := range sitemapCatalogSources {
				if err := appendRows(source.query, source.prefix, source.maker, source.priority, source.changeFreq); err != nil {
					w.Header().Set("Retry-After", "300")
					http.Error(w, "Sitemap temporarily unavailable", http.StatusServiceUnavailable)
					return
				}
			}
		}
		w.Header().Set("Content-Type", "application/xml; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=3600, s-maxage=86400")
		if r.Method == http.MethodHead {
			return
		}
		_ = xml.NewEncoder(w).Encode(set)
	}
	return robots, sitemap
}

func sitemapContentPaths(config storefront.Config) []string {
	paths := []string{"/", "/products", "/categories", "/makers", "/brands"}
	for _, page := range config.Pages {
		if page.Hidden {
			continue
		}
		prefix := "/pages/"
		if page.Legacy {
			prefix = "/"
		}
		paths = append(paths, prefix+page.Slug)
	}
	return paths
}

// Exact migrations from the previous storefront, as reported by Search
// Console. Retired products without an equivalent retain their real 404s.
var legacyPublicURLs = map[string]string{
	"/pages/about-us":              "/about",
	"/collections/frontpage":       "/products",
	"/collections/arts-collection": "/categories/arts",
	"/collections/sarees":          "/categories/textiles",
}

var sitemapCatalogSources = []struct {
	query, prefix       string
	maker               bool
	priority, changeFreq string
}{
	{`SELECT DISTINCT s.display_name FROM sellers s WHERE s.status='active' AND EXISTS (SELECT 1 FROM products p WHERE p.seller_id=s.id AND p.status='approved') ORDER BY s.display_name LIMIT 5000`, "/makers/", true, "0.7", "weekly"},
	{`SELECT b.slug FROM brands b WHERE EXISTS (SELECT 1 FROM products p JOIN sellers s ON s.id=p.seller_id WHERE p.brand_slug=b.slug AND p.status='approved' AND s.status='active') ORDER BY b.slug LIMIT 5000`, "/brands/", false, "0.8", "weekly"},
	{`SELECT c.slug FROM categories c WHERE EXISTS (SELECT 1 FROM product_categories pc JOIN products p ON p.id=pc.product_id JOIN sellers s ON s.id=p.seller_id WHERE pc.category_slug=c.slug AND p.status='approved' AND s.status='active') ORDER BY c.slug LIMIT 5000`, "/categories/", false, "0.8", "weekly"},
}

func canonicalHostRedirect(next http.Handler, canonical string, environment string) http.Handler {
	canonicalURL, _ := url.Parse(strings.TrimRight(canonical, "/"))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet || r.Method == http.MethodHead {
			if target, ok := legacyPublicURLs[r.URL.Path]; ok {
				if canonicalURL != nil && strings.EqualFold(environment, "production") {
					target = strings.TrimRight(canonicalURL.String(), "/") + target
				}
				http.Redirect(w, r, target, http.StatusMovedPermanently)
				return
			}
		}
		if canonicalURL == nil || !strings.EqualFold(environment, "production") || r.Method != http.MethodGet && r.Method != http.MethodHead {
			next.ServeHTTP(w, r)
			return
		}
		host := strings.ToLower(strings.Split(r.Host, ":")[0])
		canonicalHost := strings.ToLower(canonicalURL.Hostname())
		if host == "www."+canonicalHost {
			location := *canonicalURL
			location.Path = r.URL.Path
			location.RawQuery = r.URL.RawQuery
			http.Redirect(w, r, location.String(), http.StatusPermanentRedirect)
			return
		}
		next.ServeHTTP(w, r)
	})
}
