package main

import (
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

type rssFeed struct {
	XMLName xml.Name   `xml:"rss"`
	Version string     `xml:"version,attr"`
	GXMLNS  string     `xml:"xmlns:g,attr"`
	Channel rssChannel `xml:"channel"`
}

type rssChannel struct {
	Title       string    `xml:"title"`
	Link        string    `xml:"link"`
	Description string    `xml:"description"`
	Items       []rssItem `xml:"item"`
}

type rssItem struct {
	ID           string       `xml:"g:id"`
	Title        string       `xml:"g:title"`
	Description  string       `xml:"g:description"`
	Link         string       `xml:"g:link"`
	ImageLink    string       `xml:"g:image_link"`
	Availability string       `xml:"g:availability"`
	Price        string       `xml:"g:price"`
	Brand        string       `xml:"g:brand,omitempty"`
	Condition    string       `xml:"g:condition"`
	Shipping     *rssShipping `xml:"g:shipping,omitempty"`
}

type rssShipping struct {
	Country string `xml:"g:country"`
	Service string `xml:"g:service"`
	Price   string `xml:"g:price"`
}

func merchantFeedHandler(pool *pgxpool.Pool, publicURL string) http.HandlerFunc {
	base := strings.TrimRight(publicURL, "/")
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		feed := rssFeed{
			Version: "2.0",
			GXMLNS:  "http://base.google.com/ns/1.0",
			Channel: rssChannel{
				Title:       "Weevcrafts Product Feed",
				Link:        base,
				Description: "Handmade by India’s Makers. Shop authentic Indian arts, crafts, and handlooms.",
			},
		}

		if pool != nil {
			rows, err := pool.Query(r.Context(), `
				SELECT p.id, p.slug, p.name, COALESCE(p.description, ''),
				       COALESCE(pv.sku, ''), COALESCE(pv.price_cents, 0),
				       COALESCE(i.available_quantity, 0),
				       COALESCE(p.image_url, ''), COALESCE(s.display_name, 'Weevcrafts')
				FROM products p
				JOIN sellers s ON s.id = p.seller_id
				LEFT JOIN product_variants pv ON pv.product_id = p.id AND pv.status = 'active'
				LEFT JOIN inventory_stock i ON i.variant_id = pv.id
				WHERE p.status = 'approved' AND s.status = 'active'
				ORDER BY p.id ASC LIMIT 5000`)
			if err != nil {
				w.Header().Set("Retry-After", "300")
				http.Error(w, "Merchant feed temporarily unavailable", http.StatusServiceUnavailable)
				return
			}
			defer rows.Close()

			seen := make(map[int64]bool)
			for rows.Next() {
				var id int64
				var slug, name, desc, sku string
				var priceCents int64
				var available int
				var imageURL, brand string

				if err := rows.Scan(&id, &slug, &name, &desc, &sku, &priceCents, &available, &imageURL, &brand); err != nil {
					w.Header().Set("Retry-After", "300")
					http.Error(w, "Merchant feed temporarily unavailable", http.StatusServiceUnavailable)
					return
				}

				if seen[id] {
					continue
				}
				seen[id] = true

				availability := "out_of_stock"
				if available > 0 {
					availability = "in_stock"
				}

				img := imageURL
				if strings.HasPrefix(img, "/") {
					img = base + img
				}

				itemID := sku
				if itemID == "" {
					itemID = fmt.Sprintf("WC-%d", id)
				}

				cleanDesc := strings.ReplaceAll(desc, "#", "")
				cleanDesc = strings.ReplaceAll(cleanDesc, "*", "")
				cleanDesc = strings.TrimSpace(cleanDesc)
				if len(cleanDesc) > 5000 {
					cleanDesc = cleanDesc[:5000]
				}

				feed.Channel.Items = append(feed.Channel.Items, rssItem{
					ID:           itemID,
					Title:        name,
					Description:  cleanDesc,
					Link:         base + "/products/" + url.PathEscape(slug),
					ImageLink:    img,
					Availability: availability,
					Price:        fmt.Sprintf("%.2f INR", float64(priceCents)/100),
					Brand:        brand,
					Condition:    "new",
					Shipping: &rssShipping{
						Country: "IN",
						Service: "Standard Domestic Shipping",
						Price:   "0.00 INR",
					},
				})
			}
		}

		w.Header().Set("Content-Type", "application/xml; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=3600, s-maxage=86400")
		if r.Method == http.MethodHead {
			return
		}
		_, _ = w.Write([]byte(xml.Header))
		_ = xml.NewEncoder(w).Encode(feed)
	}
}
