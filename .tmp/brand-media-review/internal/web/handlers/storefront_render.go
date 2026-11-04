package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wecratfs/commerce/internal/adapters/postgres"
	applicationcatalog "github.com/wecratfs/commerce/internal/application/catalog"
	"github.com/wecratfs/commerce/internal/domain/storefront"
	"github.com/wecratfs/commerce/internal/ports"
	"github.com/wecratfs/commerce/internal/web/viewmodels"
)

// Pin a revision for the whole request, including API calculations. Public HTML
// remains private/no-store because it contains browser-specific CSRF tokens.
func StorefrontMiddleware(pool *pgxpool.Pool, next http.Handler, configuredMode ...string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/assets/") || strings.HasPrefix(r.URL.Path, "/health/") || strings.HasPrefix(r.URL.Path, "/media/") {
			next.ServeHTTP(w, r)
			return
		}
		rev, e := postgres.ReadPublishedStorefront(r.Context(), pool)
		if e != nil {
			http.Error(w, "The storefront is temporarily unavailable. Please try again.", 503)
			return
		}
		r = r.WithContext(storefront.WithRevision(r.Context(), rev))
		w.Header().Set("X-Storefront-Revision", fmt.Sprint(rev.ID))
		f := rev.Config.Settings.Features
		p := r.URL.Path
		if (!f.Wishlist && (strings.Contains(p, "/wishlist") || strings.HasSuffix(p, "/move-to-wishlist"))) || (!f.Reviews && r.Method != "GET" && p == "/account/reviews") || (!f.QuickView && r.Header.Get("HX-Target") == "product-quick-view-content") {
			http.Error(w, "This shopping feature is currently unavailable.", 403)
			return
		}
		if len(configuredMode) > 0 && r.Method == "POST" && (p == "/api/v1/checkout" || p == "/checkout" || strings.HasSuffix(p, "/payment")) && !rev.Config.Settings.Policy.PaymentEnabled(configuredMode[0]) {
			http.Error(w, "This payment option is currently unavailable.", 403)
			return
		}
		if r.Method == "POST" && p == "/api/v1/checkout/no-payment" && !rev.Config.Settings.Policy.PaymentEnabled("no-payment") {
			http.Error(w, "This payment option is currently unavailable.", 403)
			return
		}
		if p == "/checkout" && r.Method == "POST" {
			r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
			if e := r.ParseForm(); e != nil {
				http.Error(w, "Invalid checkout form", 400)
				return
			}
			id, e := strconv.ParseInt(r.FormValue("policy_revision"), 10, 64)
			if e != nil {
				id = -1
			}
			r = r.WithContext(storefront.WithExpectedPolicy(r.Context(), id))
		}
		next.ServeHTTP(w, r)
	})
}
func isDraftPreview(r *http.Request) bool {
	rev, ok := storefront.Current(r.Context())
	return ok && rev.Preview
}
func (h *LivePortalHandler) StorefrontPreview(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Robots-Tag", "noindex, nofollow, noarchive")
	w.Header().Set("Referrer-Policy", "same-origin")
	if r.Method != "GET" {
		w.Header().Set("Allow", "GET")
		http.Error(w, "Preview is read-only.", 405)
		return
	}
	session, ok := h.session(r)
	if !ok {
		http.Error(w, "Sign in to preview this draft.", 401)
		return
	}
	id, e := strconv.ParseInt(strings.TrimPrefix(r.URL.Path, "/storefront/preview/"), 10, 64)
	if e != nil || id <= 0 {
		http.NotFound(w, r)
		return
	}
	d, e := postgres.NewStorefrontRepository(h.pool).Draft(r.Context(), session.UserID, id)
	if e != nil {
		http.Error(w, "You no longer have access to this draft.", 403)
		return
	}
	target := r.URL.Query().Get("path")
	if target == "" {
		target = "/"
	}
	parsed, e := h.validStorefrontPreviewPath(target)
	if e != nil {
		http.Error(w, e.Error(), 422)
		return
	}
	clone := r.Clone(storefront.WithRevision(r.Context(), storefront.Revision{ID: id, Config: d.Config, Preview: true}))
	clone.URL = parsed
	clone.Header = r.Header.Clone()
	clone.Header.Del("HX-Request")
	clone.Header.Del("HX-Target")
	h.storefrontPreview.ServeHTTP(w, clone)
}
func templateKind(path string) string {
	switch {
	case strings.HasPrefix(path, "/categories"):
		return "category"
	case path == "/featured-products" || path == "/search" || path == "/products" || path == "/deals":
		return "search"
	case strings.HasPrefix(path, "/products/"):
		return "product"
	case strings.HasPrefix(path, "/makers"):
		return "maker"
	case strings.HasPrefix(path, "/brands"):
		return "brand"
	case path == "/wishlist":
		return "wishlist"
	case path == "/cart":
		return "cart"
	case path == "/checkout":
		return "checkout"
	case isAuthRoute(path):
		return "authentication"
	case strings.HasPrefix(path, "/account"):
		return "account"
	case strings.HasPrefix(path, "/orders") || path == "/tracking" || strings.HasPrefix(path, "/returns"):
		return "orders"
	}
	return ""
}
func (h *LiveStorefrontHandler) configureStorefrontPage(r *http.Request, p *viewmodels.CustomerPage) error {
	rev, e := postgres.ReadPublishedStorefront(r.Context(), h.pool)
	if e != nil {
		return e
	}
	p.MediaPositions = map[string]string{}
	p.MediaAlt = map[string]string{}
	data, _ := json.Marshal(rev.Config)
	rows, mediaErr := h.pool.Query(r.Context(), `SELECT id,focal_x,focal_y,alt FROM storefront_media WHERE strpos($1::text,'/media/storefront/'||id)>0`, string(data))
	if mediaErr != nil {
		return mediaErr
	}
	for rows.Next() {
		var id, alt string
		var x, y int
		if e := rows.Scan(&id, &x, &y, &alt); e != nil {
			rows.Close()
			return e
		}
		ref := "/media/storefront/" + id
		p.MediaPositions[ref] = fmt.Sprintf("%d%% %d%%", x, y)
		p.MediaAlt[ref] = alt
	}
	rows.Close()
	if e := rows.Err(); e != nil {
		return e
	}
	p.Storefront = &rev.Config
	p.StorefrontRevision = rev.ID
	p.StorefrontPreview = rev.Preview
	if len(rev.Config.Categories) > 0 {
		counts := map[string]int{}
		for _, c := range p.Categories {
			counts[c.Slug] = c.Products
		}
		p.Categories = nil
		for _, c := range rev.Config.OrderedCategories() {
			p.Categories = append(p.Categories, viewmodels.Category{Slug: c.Slug, Name: c.Name, Description: c.Description, ImageURL: c.Image, DisplayOrder: c.Order, ShowInNavigation: c.Visible, Products: counts[c.Slug], ImagePosition: p.MediaPositions[c.Image]})
		}
	}
	if !rev.Config.Settings.Policy.PaymentEnabled(p.PaymentMode) {
		p.PaymentReady = false
	}
	if !rev.Config.Settings.Policy.PaymentEnabled("no-payment") {
		p.NoPaymentDemo = false
	}
	p.SupportEmail = rev.Config.Settings.SupportEmail
	p.ShippingInformation = rev.Config.Settings.ShippingInformation
	p.SocialLinks = nil
	for _, l := range rev.Config.Settings.Social {
		p.SocialLinks = append(p.SocialLinks, ports.SocialLink{Name: l.Label, URL: l.URL})
	}
	t := rev.Config.Template(templateKind(p.Route), p.Category, strings.TrimPrefix(p.Route, "/products/"))
	p.StorefrontTemplate = t
	if t.Title != "" {
		p.Title = t.Title
	}
	if t.Description != "" {
		p.Description = t.Description
	}
	pathSlug := strings.TrimPrefix(strings.TrimPrefix(p.Route, "/pages/"), "/")
	for _, page := range rev.Config.Pages {
		if page.Slug == pathSlug && (strings.HasPrefix(p.Route, "/pages/") || isInfoRoute(p.Route)) {
			if page.Hidden {
				return applicationcatalog.ErrProductNotFound
			}
			copy := page
			p.StorefrontPage = &copy
			p.Title = page.Title
			p.Description = page.Description
		}
	}
	return nil
}
func (h *LiveStorefrontHandler) loadConfiguredSections(r *http.Request, p *viewmodels.CustomerPage) error {
	p.SectionProducts = map[string][]viewmodels.Product{}
	if p.Storefront == nil {
		return nil
	}
	sections := p.Storefront.Homepage
	if p.Route != "/" {
		sections = p.StorefrontTemplate.Sections
		if p.StorefrontPage != nil {
			sections = append(append([]storefront.Section{}, sections...), p.StorefrontPage.Sections...)
		}
	}
	if p.HomeSelectedCategory == "" {
		p.HomeSelectedCategory = p.Category
		for _, c := range p.Categories {
			if c.ShowInNavigation && c.Slug == r.URL.Query().Get("home_category") {
				p.HomeSelectedCategory = c.Slug
				break
			}
			if p.HomeSelectedCategory == "" && c.ShowInNavigation {
				p.HomeSelectedCategory = c.Slug
			}
		}
	}
	for _, s := range sections {
		if s.Hidden || s.Type != "products" {
			continue
		}
		sel := s.Selection
		category := sel.Category
		if sel.Mode == "category" && category == "" {
			category = p.HomeSelectedCategory
		}
		options := ports.ProductSearchOptions{CategorySlug: category, DatabaseOnly: true, InStock: sel.Available, Limit: max(1, min(24, sel.Limit)), MinPriceCents: sel.MinPrice, MaxPriceCents: sel.MaxPrice, ProductIDs: sel.Products, SelectedOnly: sel.Mode == "selected", Featured: sel.Mode == "featured"}
		if options.Featured {
			options.InStock = true
		}
		if sel.Mode == "newest" {
			options.Sort = "newest"
		}
		products, _, _, e := h.search.SearchWithFilters(r.Context(), options)
		if e != nil {
			return e
		}
		p.SectionProducts[s.ID] = h.productViews(r.Context(), products)
		for i := range p.SectionProducts[s.ID] {
			p.SectionProducts[s.ID][i].CSRFToken = p.CSRFToken
		}

	}
	return nil
}

func (h *LivePortalHandler) SetStorefrontPreview(handler *LiveStorefrontHandler) {
	h.storefrontPreview = handler
}

func liveTotalsWithPolicy(items []viewmodels.CartItem, p storefront.Policy) (string, string, string, string) {
	var subtotal int64
	for _, item := range items {
		subtotal += item.Product.PriceCents * int64(item.Quantity)
	}
	shipping := p.Shipping(subtotal)
	return storefrontMoney(subtotal), storefrontMoney(shipping), storefrontMoney(0), storefrontMoney(subtotal + shipping)
}

func (h *LiveStorefrontHandler) prepareReadOnlyPreview(r *http.Request, p *viewmodels.CustomerPage) error {
	if p.Route == "/cart" || p.Route == "/checkout" || p.Route == "/wishlist" {
		source, _, _, e := h.search.SearchWithFilters(r.Context(), ports.ProductSearchOptions{InStock: true, DatabaseOnly: true, Limit: 1})
		if e != nil {
			return e
		}
		products := h.productViews(r.Context(), source)
		if len(products) > 0 {
			if p.Route == "/wishlist" {
				p.SavedProducts = products
				p.WishlistCount = len(products)
			} else {
				p.Cart = []viewmodels.CartItem{{Product: products[0], Quantity: 1, Delivery: products[0].DeliveryLabel}}
				p.CartCount = 1
				p.ItemsTotal, p.ShippingTotal, p.DiscountTotal, p.Total = liveTotalsWithPolicy(p.Cart, p.Storefront.Settings.Policy)
			}
		}
	}
	if p.Route == "/cart" || p.Route == "/checkout" || p.Route == "/wishlist" || isCustomerAccountRoute(p.Route) || p.Route == "/orders" {
		p.Authenticated = true
		p.UserName = "Customer preview"
		p.IsCustomer = true
	}
	return nil
}
