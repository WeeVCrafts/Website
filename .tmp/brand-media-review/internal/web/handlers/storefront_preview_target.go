package handlers

import (
	"fmt"
	"github.com/wecratfs/commerce/internal/web/viewmodels"
	"net/http"
	"net/url"
	"strings"
)

func (h *LivePortalHandler) storefrontPreviewTarget(r *http.Request, m *viewmodels.StorefrontEditor) {
	path := "/"
	if m.Tab == "settings" {
		path = "/checkout"
	}
	if m.Tab == "categories" && m.Category.Slug != "" {
		path = "/categories/" + m.Category.Slug
	}
	if (m.Tab == "pages" || m.Tab == "page-sections") && m.Page.Slug != "" {
		path = "/pages/" + m.Page.Slug
		if isInfoRoute("/" + m.Page.Slug) {
			path = "/" + m.Page.Slug
		}
	}
	if m.Tab == "templates" || m.Tab == "template-sections" {
		t := m.Template
		switch t.Kind {
		case "category":
			path = "/categories"
			if strings.HasPrefix(t.Match, "category:") {
				path = "/categories/" + strings.TrimPrefix(t.Match, "category:")
			}
		case "search":
			path = "/products"
		case "product":
			path = "/products"
			slug := strings.TrimPrefix(t.Match, "product:")
			if !strings.HasPrefix(t.Match, "product:") {
				_ = h.pool.QueryRow(r.Context(), `SELECT slug FROM products WHERE status='approved' ORDER BY featured_rank,id LIMIT 1`).Scan(&slug)
			}
			if slug != "" {
				path = "/products/" + slug
			}
		case "maker":
			path = "/makers"
		case "brand":
			path = "/brands"
		case "authentication":
			path = "/login"
		case "account":
			path = "/account"
		case "orders":
			path = "/orders"
		case "wishlist", "cart", "checkout":
			path = "/" + t.Kind
		}
	}
	m.PreviewURL = "/storefront/preview/" + fmt.Sprint(m.Draft.ID) + "?path=" + url.QueryEscape(path)
	if m.Tab == "homepage" && m.Section.ID != "" {
		m.PreviewURL += "#section-" + m.Section.ID
	}
}
