package handlers

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/jackc/pgx/v5/pgxpool"

	applicationauth "github.com/wecratfs/commerce/internal/application/auth"
	applicationcatalog "github.com/wecratfs/commerce/internal/application/catalog"
	applicationcommerce "github.com/wecratfs/commerce/internal/application/commerce"
	domaincatalog "github.com/wecratfs/commerce/internal/domain/catalog"
	domaincommerce "github.com/wecratfs/commerce/internal/domain/commerce"
	domainidentity "github.com/wecratfs/commerce/internal/domain/identity"
	"github.com/wecratfs/commerce/internal/web/viewmodels"
	"github.com/wecratfs/commerce/web/components"
	"github.com/wecratfs/commerce/web/pages"
)

// LiveStorefrontHandler keeps customer-facing pages in the shared web/pages
// visual system while sourcing their state from the live application services.
// The transport/web handler remains responsible for mutations and API routes.
type LiveStorefrontHandler struct {
	catalog          *applicationcatalog.Service
	search           *applicationcatalog.SearchService
	cart             *applicationcommerce.CartService
	orders           *applicationcommerce.OrderService
	sessions         *applicationauth.SessionService
	pool             *pgxpool.Pool
	paymentMode      string
	paymentPublicKey string
	paymentReady     bool
	fallback         http.Handler
}

func NewLiveStorefrontHandler(catalog *applicationcatalog.Service, search *applicationcatalog.SearchService, cart *applicationcommerce.CartService, orders *applicationcommerce.OrderService, sessions *applicationauth.SessionService, pool *pgxpool.Pool, paymentMode, paymentPublicKey string, paymentConfigured bool, fallback http.Handler) *LiveStorefrontHandler {
	mode := strings.ToLower(strings.TrimSpace(paymentMode))
	ready := paymentReady(mode, paymentConfigured)
	return &LiveStorefrontHandler{
		catalog: catalog, search: search, cart: cart, orders: orders, sessions: sessions, pool: pool,
		paymentMode: mode, paymentPublicKey: paymentPublicKey, paymentReady: ready, fallback: fallback,
	}
}

func paymentReady(mode string, paymentConfigured bool) bool {
	return mode == "local" || mode == "razorpay" && paymentConfigured
}

func (h *LiveStorefrontHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/account/profile" && r.Method == http.MethodPost {
		h.updateProfile(w, r)
		return
	}
	if r.URL.Path == "/account/addresses" && r.Method == http.MethodPost {
		h.saveAddress(w, r)
		return
	}
	if r.URL.Path == "/account/addresses/delete" && r.Method == http.MethodPost {
		h.deleteAddress(w, r)
		return
	}
	if r.URL.Path == "/ui/search-suggest" && r.Method == http.MethodGet {
		h.searchSuggestions(w, r)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/ui/seller-follow/") {
		h.sellerFollowMutation(w, r)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/ui/cart/") || strings.HasPrefix(r.URL.Path, "/ui/wishlist/") {
		h.uiMutation(w, r)
		return
	}
	if !h.ownsRoute(r.URL.Path) || r.Method != http.MethodGet {
		h.fallback.ServeHTTP(w, r)
		return
	}
	userID, _, authenticated := h.session(r)
	if requiresCustomerSession(r.URL.Path) && !authenticated {
		next := r.URL.RequestURI()
		http.Redirect(w, r, "/login?next="+url.QueryEscape(next), http.StatusSeeOther)
		return
	}
	if requiresCustomerSession(r.URL.Path) && !h.customerRole(userID, r) {
		roles := h.userRoles(r, userID)
		destination := applicationauth.LoginDestination(roles)
		if destination == "/account" {
			destination = "/workspaces"
		}
		http.Redirect(w, r, destination, http.StatusSeeOther)
		return
	}
	if isAuthRoute(r.URL.Path) && authenticated {
		roles := h.userRoles(r, userID)
		destination := applicationauth.LoginDestination(roles)
		if customerRoleAllowed(roles) {
			destination = safeCustomerNext(r.URL.Query().Get("next"), destination)
		}
		http.Redirect(w, r, destination, http.StatusSeeOther)
		return
	}
	page, err := h.page(r)
	if err != nil {
		if errors.Is(err, applicationcatalog.ErrProductNotFound) {
			http.NotFound(w, r)
			return
		}
		page = viewmodels.CustomerPage{
			Route: r.URL.Path, Title: "We are having a little trouble", Brand: "WeeVCrafts",
			Tagline: "Handmade Today. A Kinder Tomorrow.", Description: "Please try again in a moment.",
			Notice: "We could not load this page right now. Please try again in a moment.",
		}
		w.WriteHeader(http.StatusServiceUnavailable)
		h.render(w, r, pages.Empty(page))
		return
	}
	if strings.HasPrefix(r.URL.Path, "/products/") {
		variants, variantsErr := h.customerProductVariants(r, page.Product.Slug)
		if variantsErr != nil {
			http.Error(w, "Product variants are temporarily unavailable", http.StatusServiceUnavailable)
			return
		}
		page.Product.Variants = variants
		if len(variants) > 0 {
			first := variants[0]
			page.Product.VariantID, _ = strconv.ParseInt(first.ID, 10, 64)
			page.Product.PriceCents, _ = strconv.ParseInt(first.PriceCents, 10, 64)
			page.Product.Price, page.Product.CompareAt, page.Product.Discount = first.Price, first.CompareAt, first.Discount
			page.Product.CompareAtCents, _ = strconv.ParseInt(first.CompareAtCents, 10, 64)
			page.Product.HasCompareAt = first.HasCompareAt
			page.Product.Available, _ = strconv.Atoi(first.Available)
			page.Product.InStock = first.InStock
			page.Product.Badge = map[bool]string{true: "In stock", false: "Sold out"}[first.InStock]
		}
		media, mediaErr := productMediaViews(r.Context(), h.pool, page.Product.ID)
		if mediaErr != nil {
			http.Error(w, "Product media are temporarily unavailable", http.StatusServiceUnavailable)
			return
		}
		page.Product.Media = media
	}
	var component templ.Component
	switch {
	case r.URL.Path == "/":
		component = pages.Home(page)
	case r.URL.Path == "/categories":
		component = pages.Categories(page)
	case r.URL.Path == "/brands":
		component = pages.Brands(page)
	case strings.HasPrefix(r.URL.Path, "/brands/"):
		component = pages.BrandDetail(page, liveBrand(r.URL.Path, page.Brands))
	case r.URL.Path == "/products" || r.URL.Path == "/search" || r.URL.Path == "/deals":
		component = pages.Listing(page)
	case strings.HasPrefix(r.URL.Path, "/products/"):
		component = pages.ProductDetail(page)
	case strings.HasPrefix(r.URL.Path, "/makers/"):
		seller := liveSeller(r.URL.Path, page.Sellers)
		if seller.ID == 0 {
			http.NotFound(w, r)
			return
		}
		if page.CanFollowSeller {
			seller.Following = h.sellerFollowed(r.Context(), userID, seller.ID)
		}
		component = pages.SellerStore(page, seller)
	case isAuthRoute(r.URL.Path):
		component = pages.Auth(page)
	case r.URL.Path == "/account":
		component = pages.LiveAccount(page)
	case strings.HasPrefix(r.URL.Path, "/account/"):
		component = pages.LiveAccountDetail(page)
	case r.URL.Path == "/wishlist":
		component = pages.Wishlist(page)
	case r.URL.Path == "/cart":
		component = pages.Cart(page)
	case r.URL.Path == "/checkout":
		if page.CartCount == 0 {
			http.Redirect(w, r, "/cart", http.StatusSeeOther)
			return
		}
		component = pages.Checkout(page)
	case r.URL.Path == "/orders":
		component = pages.LiveOrders(page)
	case strings.HasPrefix(r.URL.Path, "/orders/"):
		if r.URL.Path == "/orders/invoice" {
			page.Title = "Invoice"
			page.Notice = "Invoices are available after payment is confirmed."
			component = pages.Empty(page)
		} else {
			if page.SelectedOrder.Number == "" {
				http.NotFound(w, r)
				return
			}
			component = pages.OrderDetail(page)
		}
	case r.URL.Path == "/tracking":
		component = pages.LiveTracking(page)
	case r.URL.Path == "/returns":
		if tab := r.URL.Query().Get("tab"); tab != "" {
			component = pages.ReturnsFiltered(page, tab)
		} else {
			component = pages.Returns(page)
		}
	case strings.HasPrefix(r.URL.Path, "/returns/"):
		page.Title = "Return label"
		page.Notice = "A return label is available after your return request is approved."
		component = pages.Empty(page)
	case r.URL.Path == "/payments":
		component = pages.Payments(page)
	case strings.HasPrefix(r.URL.Path, "/payments/"):
		switch strings.TrimPrefix(r.URL.Path, "/payments/") {
		case "success":
			component = pages.PaymentStatus(page, "success")
		case "failed":
			component = pages.PaymentStatus(page, "failed")
		default:
			component = pages.PaymentStatus(page, "pending")
		}
	case isInfoRoute(r.URL.Path):
		component = pages.InfoPage(page)
	default:
		h.fallback.ServeHTTP(w, r)
		return
	}
	h.render(w, r, component)
}

func (h *LiveStorefrontHandler) updateProfile(w http.ResponseWriter, r *http.Request) {
	userID, csrf, ok := h.session(r)
	if !ok {
		http.Redirect(w, r, "/login?next=%2Faccount%2Fprofile", http.StatusSeeOther)
		return
	}
	if !h.customerRole(userID, r) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid profile form", http.StatusBadRequest)
		return
	}
	providedToken := r.Header.Get("X-CSRF-Token")
	if providedToken == "" {
		providedToken = r.Form.Get("csrf_token")
	}
	if subtle.ConstantTimeCompare([]byte(csrf), []byte(providedToken)) != 1 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	name := strings.TrimSpace(r.Form.Get("name"))
	if len([]rune(name)) < 2 || len([]rune(name)) > 100 {
		http.Error(w, "name must contain 2 to 100 characters", http.StatusUnprocessableEntity)
		return
	}
	if _, err := h.pool.Exec(r.Context(), `UPDATE users SET display_name = $1, updated_at = NOW() WHERE id = $2 AND status = 'active'`, name, userID); err != nil {
		http.Error(w, "profile could not be saved", http.StatusServiceUnavailable)
		return
	}
	http.Redirect(w, r, "/account/profile?notice=Profile+updated", http.StatusSeeOther)
}

func (h *LiveStorefrontHandler) saveAddress(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.customerMutationActor(w, r)
	if !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid address form", http.StatusBadRequest)
		return
	}
	address, err := applicationcommerce.NormalizeAddressInput(domaincommerce.AddressInput{
		RecipientName: r.Form.Get("recipient_name"), Phone: r.Form.Get("phone"),
		Line1: r.Form.Get("line1"), Line2: r.Form.Get("line2"), City: r.Form.Get("city"),
		State: r.Form.Get("state"), PostalCode: r.Form.Get("postal_code"), CountryCode: r.Form.Get("country_code"),
	})
	if err != nil {
		http.Error(w, "shipping address is invalid", http.StatusUnprocessableEntity)
		return
	}
	addressIDText := strings.TrimSpace(r.Form.Get("address_id"))
	if addressIDText == "" {
		_, err = h.pool.Exec(r.Context(), `
			INSERT INTO addresses (user_id, recipient_name, phone, line1, line2, city, state, postal_code, country_code)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
			userID, address.RecipientName, address.Phone, address.Line1, address.Line2, address.City, address.State, address.PostalCode, address.CountryCode)
	} else {
		var addressID int64
		addressID, err = strconv.ParseInt(addressIDText, 10, 64)
		if err != nil || addressID <= 0 {
			http.Error(w, "invalid address id", http.StatusBadRequest)
			return
		}
		result, updateErr := h.pool.Exec(r.Context(), `
			UPDATE addresses
			SET recipient_name = $1, phone = $2, line1 = $3, line2 = $4, city = $5, state = $6, postal_code = $7, country_code = $8, updated_at = NOW()
			WHERE id = $9 AND user_id = $10`,
			address.RecipientName, address.Phone, address.Line1, address.Line2, address.City, address.State, address.PostalCode, address.CountryCode, addressID, userID)
		err = updateErr
		if err == nil && result.RowsAffected() == 0 {
			http.NotFound(w, r)
			return
		}
	}
	if err != nil {
		http.Error(w, "address could not be saved", http.StatusServiceUnavailable)
		return
	}
	http.Redirect(w, r, "/account/addresses?notice=Address+saved", http.StatusSeeOther)
}

func (h *LiveStorefrontHandler) deleteAddress(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.customerMutationActor(w, r)
	if !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid address form", http.StatusBadRequest)
		return
	}
	addressID, err := strconv.ParseInt(strings.TrimSpace(r.Form.Get("address_id")), 10, 64)
	if err != nil || addressID <= 0 {
		http.Error(w, "invalid address id", http.StatusBadRequest)
		return
	}
	result, err := h.pool.Exec(r.Context(), `DELETE FROM addresses WHERE id = $1 AND user_id = $2`, addressID, userID)
	if err != nil {
		http.Error(w, "address could not be deleted", http.StatusServiceUnavailable)
		return
	}
	if result.RowsAffected() == 0 {
		http.NotFound(w, r)
		return
	}
	http.Redirect(w, r, "/account/addresses?notice=Address+deleted", http.StatusSeeOther)
}

func (h *LiveStorefrontHandler) customerMutationActor(w http.ResponseWriter, r *http.Request) (int64, bool) {
	userID, csrf, authenticated := h.session(r)
	if !authenticated {
		http.Redirect(w, r, "/login?next="+url.QueryEscape(r.URL.RequestURI()), http.StatusSeeOther)
		return 0, false
	}
	if !h.customerRole(userID, r) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return 0, false
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return 0, false
	}
	providedToken := r.Header.Get("X-CSRF-Token")
	if providedToken == "" {
		providedToken = r.Form.Get("csrf_token")
	}
	if subtle.ConstantTimeCompare([]byte(csrf), []byte(providedToken)) != 1 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return 0, false
	}
	return userID, true
}

func (h *LiveStorefrontHandler) ownsRoute(path string) bool {
	return path == "/" || path == "/categories" || path == "/brands" || strings.HasPrefix(path, "/brands/") || path == "/products" || path == "/search" || path == "/deals" || strings.HasPrefix(path, "/products/") || strings.HasPrefix(path, "/makers/") || isAuthRoute(path) || path == "/account" || strings.HasPrefix(path, "/account/") || path == "/wishlist" || path == "/cart" || path == "/checkout" || path == "/orders" || strings.HasPrefix(path, "/orders/") || path == "/tracking" || path == "/returns" || strings.HasPrefix(path, "/returns/") || path == "/payments" || strings.HasPrefix(path, "/payments/") || isInfoRoute(path)
}

func isAuthRoute(path string) bool {
	switch path {
	case "/login", "/register", "/forgot-password", "/reset-password", "/verify-email":
		return true
	default:
		return false
	}
}

func requiresCustomerSession(path string) bool {
	return path == "/account" || strings.HasPrefix(path, "/account/") || path == "/wishlist" || path == "/cart" || path == "/checkout" || path == "/orders" || strings.HasPrefix(path, "/orders/") || path == "/tracking" || path == "/returns" || strings.HasPrefix(path, "/returns/") || path == "/payments" || strings.HasPrefix(path, "/payments/")
}

func isInfoRoute(path string) bool {
	switch path {
	case "/faq", "/contact", "/shipping", "/size-guide", "/terms", "/privacy", "/about", "/our-story", "/sustainability", "/press", "/careers":
		return true
	default:
		return false
	}
}

func safeCustomerNext(candidate, fallback string) string {
	candidate = strings.TrimSpace(candidate)
	if candidate == "" || !strings.HasPrefix(candidate, "/") || strings.HasPrefix(candidate, "//") || strings.Contains(candidate, "\\") {
		return fallback
	}
	return candidate
}

func (h *LiveStorefrontHandler) page(r *http.Request) (viewmodels.CustomerPage, error) {
	userID, csrf, authenticated := h.session(r)
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	category := strings.TrimSpace(r.URL.Query().Get("category"))
	sortMode := strings.TrimSpace(r.URL.Query().Get("sort"))
	if r.URL.Path == "/deals" && category == "" {
		category = "offers"
	}

	products, err := h.products(r.Context(), r.URL.Path, query, category, sortMode)
	if err != nil {
		return viewmodels.CustomerPage{}, err
	}
	if strings.HasPrefix(r.URL.Path, "/brands/") {
		products, err = h.productsForBrand(r.Context(), strings.TrimPrefix(r.URL.Path, "/brands/"), csrf)
		if err != nil {
			return viewmodels.CustomerPage{}, err
		}
	}
	if strings.HasPrefix(r.URL.Path, "/makers/") {
		products, err = h.productsForSeller(r.Context(), strings.TrimPrefix(r.URL.Path, "/makers/"), csrf, sortMode)
		if err != nil {
			return viewmodels.CustomerPage{}, err
		}
	}
	if requestedBrand := strings.TrimSpace(r.URL.Query().Get("brand")); requestedBrand != "" {
		products, err = h.productsForBrand(r.Context(), requestedBrand, csrf)
		if err != nil {
			return viewmodels.CustomerPage{}, err
		}
	}
	page := viewmodels.CustomerPage{
		Route: r.URL.Path, Title: liveTitle(r.URL.Path), Query: query, Category: category, Sort: sortMode,
		Notice: r.URL.Query().Get("notice"), Filters: r.URL.Query(), Brand: "WeeVCrafts",
		Tagline: "Handmade Today. A Kinder Tomorrow.", Description: "Authentic Indian arts, crafts and sarees, made with care.",
		PaymentMode: h.paymentMode, PaymentPublicKey: h.paymentPublicKey, PaymentReady: h.paymentReady,
		Products: products, Categories: h.categories(r.Context()), Sellers: h.sellers(r.Context()), Brands: h.brands(r.Context()),
		Authenticated: authenticated, CanCreateSupport: authenticated && h.customerRole(userID, r), CanFollowSeller: authenticated && h.customerRole(userID, r), CSRFToken: csrf, Now: time.Now(), Page: 1, Pages: 1,
	}
	if strings.HasPrefix(r.URL.Path, "/products/") {
		product, productErr := h.catalog.Get(r.Context(), strings.TrimPrefix(r.URL.Path, "/products/"))
		if productErr != nil {
			return viewmodels.CustomerPage{}, productErr
		}
		page.Product = liveProduct(product, csrf)
		page.Product.Seller = h.sellerName(r.Context(), product.Slug)
		page.Product.SellerSlug = liveSlug(page.Product.Seller)
		page.Products = []viewmodels.Product{page.Product}
	}
	if authenticated && h.cart != nil {
		page.Cart, page.CartCount = h.liveCart(r.Context(), userID, csrf)
		page.SavedProducts, page.WishlistCount = h.liveWishlist(r.Context(), userID, csrf)
		page.ItemsTotal, page.ShippingTotal, page.DiscountTotal, page.Total = liveTotals(page.Cart)
	}
	if authenticated && needsAccountData(r.URL.Path) {
		if err := h.loadCustomerData(r.Context(), userID, &page); err != nil {
			return viewmodels.CustomerPage{}, err
		}
	}
	return page, nil
}

func needsAccountData(path string) bool {
	return path == "/account" || path == "/checkout" || path == "/orders" || strings.HasPrefix(path, "/orders/") || path == "/tracking" || path == "/returns" || path == "/payments" || strings.HasPrefix(path, "/account/")
}

func (h *LiveStorefrontHandler) loadCustomerData(ctx context.Context, userID int64, page *viewmodels.CustomerPage) error {
	if h.pool == nil || userID <= 0 {
		return fmt.Errorf("customer account data is unavailable")
	}
	var createdAt time.Time
	var err error
	var roleSlugs []string
	if err := h.pool.QueryRow(ctx, `SELECT display_name, email, created_at, COALESCE(ARRAY(SELECT role_slug FROM user_roles WHERE user_id = $1 ORDER BY role_slug), ARRAY[]::text[]) FROM users WHERE id = $1 AND status = 'active'`, userID).Scan(&page.UserName, &page.UserEmail, &createdAt, &roleSlugs); err != nil {
		return err
	}
	if strings.TrimSpace(page.UserName) == "" {
		page.UserName = page.UserEmail
	}
	page.MemberSince = createdAt.Format("2 Jan 2006")
	page.RoleLabel = customerPortalRoleLabel(roleSlugs)
	page.PortalLabel = "Customer Account"
	if page.Route == "/account/support" {
		page.SupportTickets, err = h.customerSupportTickets(ctx, userID, page.UserName)
		if err != nil {
			return err
		}
	}
	if err := h.pool.QueryRow(ctx, `SELECT COUNT(*) FROM orders WHERE user_id = $1`, userID).Scan(&page.OrderCount); err != nil {
		return err
	}
	addresses, err := h.pool.Query(ctx, `SELECT id, recipient_name, phone, line1, line2, city, state, postal_code, country_code FROM addresses WHERE user_id = $1 ORDER BY id DESC`, userID)
	if err != nil {
		return err
	}
	page.Addresses = make([]viewmodels.Address, 0)
	for addresses.Next() {
		var address viewmodels.Address
		var addressID int64
		if err := addresses.Scan(&addressID, &address.Name, &address.Phone, &address.Line1, &address.Line2, &address.City, &address.State, &address.PostalCode, &address.CountryCode); err != nil {
			addresses.Close()
			return err
		}
		address.ID = strconv.FormatInt(addressID, 10)
		address.Lines = strings.Join(nonEmptyAddressLines(address), ", ")
		page.Addresses = append(page.Addresses, address)
	}
	if err := addresses.Err(); err != nil {
		addresses.Close()
		return err
	}
	addresses.Close()
	if len(page.Addresses) > 0 {
		page.Address = page.Addresses[0]
	}

	if h.orders != nil && (page.Route == "/account" || page.Route == "/orders" || strings.HasPrefix(page.Route, "/orders/") || page.Route == "/tracking") {
		orders, err := h.orders.List(ctx, userID, 20)
		if err != nil {
			return err
		}
		page.Orders = make([]viewmodels.Order, 0, len(orders))
		for _, order := range orders {
			detail, err := h.orders.Get(ctx, userID, order.OrderNumber)
			if err != nil {
				return err
			}
			view, err := h.customerOrderView(ctx, detail)
			if err != nil {
				return err
			}
			page.Orders = append(page.Orders, view)
		}
		if strings.HasPrefix(page.Route, "/orders/") && page.Route != "/orders/invoice" {
			orderNumber := strings.TrimPrefix(page.Route, "/orders/")
			detail, err := h.orders.Get(ctx, userID, orderNumber)
			if err != nil {
				return err
			}
			page.SelectedOrder, err = h.customerOrderView(ctx, detail)
			if err != nil {
				return err
			}
		}
		if page.Route == "/tracking" {
			orderNumber := strings.TrimSpace(page.Filters.Get("order"))
			if orderNumber != "" {
				detail, err := h.orders.Get(ctx, userID, orderNumber)
				if err != nil {
					return err
				}
				page.SelectedOrder, err = h.customerOrderView(ctx, detail)
				if err != nil {
					return err
				}
			} else if len(page.Orders) > 0 {
				page.SelectedOrder = page.Orders[0]
			}
		}
	}
	if page.Route == "/returns" {
		page.Returns, err = h.customerReturns(ctx, userID)
		if err != nil {
			return err
		}
	}
	if page.Route == "/payments" {
		page.PaymentActivity, err = h.customerPaymentActivity(ctx, userID)
		if err != nil {
			return err
		}
	}
	return nil
}

func nonEmptyAddressLines(address viewmodels.Address) []string {
	parts := []string{address.Line1, address.Line2, address.City, address.State, address.PostalCode, address.CountryCode}
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if part = strings.TrimSpace(part); part != "" {
			result = append(result, part)
		}
	}
	return result
}

func customerPortalRoleLabel(roles []string) string {
	for _, role := range roles {
		if role == string(domainidentity.RoleCustomer) {
			return "Customer"
		}
	}
	return "Account"
}

func (h *LiveStorefrontHandler) customerSupportTickets(ctx context.Context, userID int64, customerName string) ([]viewmodels.CustomerSupportTicket, error) {
	rows, err := h.pool.Query(ctx, `SELECT t.id,t.ticket_number,t.subject,t.message,t.status,t.priority,t.created_at,COALESCE(o.order_number,'') FROM support_tickets t LEFT JOIN orders o ON o.id=t.order_id WHERE t.user_id=$1 ORDER BY t.created_at DESC,t.id DESC LIMIT 100`, userID)
	if err != nil {
		return nil, err
	}
	tickets := make([]viewmodels.CustomerSupportTicket, 0)
	for rows.Next() {
		var item viewmodels.CustomerSupportTicket
		var created time.Time
		if err := rows.Scan(&item.ID, &item.Number, &item.Subject, &item.InitialMessage, &item.Status, &item.Priority, &created, &item.OrderNumber); err != nil {
			rows.Close()
			return nil, err
		}
		item.Created = created.Format("2 Jan 2006, 15:04")
		item.Messages = append(item.Messages, viewmodels.CustomerSupportMessage{Author: customerName, Role: "You", Body: item.InitialMessage, Created: item.Created})
		tickets = append(tickets, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	for index := range tickets {
		messages, err := h.pool.Query(ctx, `SELECT u.display_name,m.author_role,m.body,m.created_at FROM support_ticket_messages m JOIN users u ON u.id=m.author_id WHERE m.ticket_id=$1 ORDER BY m.created_at,m.id`, tickets[index].ID)
		if err != nil {
			return nil, err
		}
		for messages.Next() {
			var author, authorRole, body string
			var created time.Time
			if err := messages.Scan(&author, &authorRole, &body, &created); err != nil {
				messages.Close()
				return nil, err
			}
			role := "You"
			if authorRole == "support_agent" {
				role = "Support agent"
			}
			tickets[index].Messages = append(tickets[index].Messages, viewmodels.CustomerSupportMessage{Author: author, Role: role, Body: body, Created: created.Format("2 Jan 2006, 15:04")})
		}
		if err := messages.Err(); err != nil {
			messages.Close()
			return nil, err
		}
		messages.Close()
	}
	return tickets, nil
}

func (h *LiveStorefrontHandler) customerOrderView(ctx context.Context, order domaincommerce.OrderDetail) (viewmodels.Order, error) {
	status := displayOrderStatus(order.Status)
	payment := displayPaymentStatus(order.PaymentStatus, order.Status)
	view := viewmodels.Order{
		Number: order.OrderNumber, Status: status, State: order.Status, Payment: payment,
		Date: order.CreatedAt.Format("2 Jan 2006"), Total: storefrontMoney(order.TotalCents),
		CanRequestReturn: order.Status == "paid" || order.Status == "processing" || order.Status == "shipped" || order.Status == "delivered",
	}
	if err := h.pool.QueryRow(ctx, `SELECT COALESCE((SELECT status FROM return_requests WHERE order_id = $1), '')`, order.ID).Scan(&view.ReturnStatus); err != nil {
		return viewmodels.Order{}, err
	}
	if view.ReturnStatus != "" {
		view.CanRequestReturn = false
	}
	rows, err := h.pool.Query(ctx, `
		SELECT COALESCE(p.slug, ''), oi.product_name, oi.unit_price_cents, oi.quantity,
		       COALESCE(p.image_url, ''), COALESCE(s.display_name, 'Independent maker')
		FROM order_items oi
		LEFT JOIN product_variants v ON v.id = oi.variant_id
		LEFT JOIN products p ON p.id = v.product_id
		LEFT JOIN sellers s ON s.id = p.seller_id
		WHERE oi.order_id = $1
		ORDER BY oi.id`, order.ID)
	if err != nil {
		return viewmodels.Order{}, err
	}
	defer rows.Close()
	view.Items = make([]viewmodels.CartItem, 0, len(order.Items))
	for rows.Next() {
		var slug, name, image, seller string
		var cents int64
		var quantity int
		if err := rows.Scan(&slug, &name, &cents, &quantity, &image, &seller); err != nil {
			return viewmodels.Order{}, err
		}
		if image == "" {
			image = "/assets/images/customer/maker-mithila.webp"
		}
		product := viewmodels.Product{Slug: slug, Name: name, Seller: seller, ImageURL: image, PriceCents: cents, Price: storefrontMoney(cents)}
		view.Items = append(view.Items, viewmodels.CartItem{Product: product, Quantity: quantity, Delivery: "4 - 6 days"})
	}
	return view, rows.Err()
}

func (h *LiveStorefrontHandler) customerReturns(ctx context.Context, userID int64) ([]viewmodels.Return, error) {
	rows, err := h.pool.Query(ctx, `
		SELECT rr.id::text,
		       CASE WHEN pr.status = 'processed' THEN 'Refunded'
	            WHEN rr.status = 'pending' THEN 'Pending review'
	            WHEN rr.status = 'approved' THEN 'Approved'
	            WHEN rr.status = 'received' THEN 'Item received'
	            WHEN rr.status = 'rejected' THEN 'Declined'
	            ELSE INITCAP(REPLACE(rr.status, '_', ' ')) END,
		       rr.reason, TO_CHAR(rr.created_at, 'DD Mon YYYY'),
		       COALESCE((SELECT oi.product_name FROM order_items oi WHERE oi.order_id = o.id ORDER BY oi.id LIMIT 1), 'Order return'),
		       COALESCE((SELECT p.image_url FROM order_items oi JOIN product_variants v ON v.id = oi.variant_id JOIN products p ON p.id = v.product_id WHERE oi.order_id = o.id ORDER BY oi.id LIMIT 1), '/assets/images/customer/maker-mithila.webp'),
		       o.order_number, o.total_cents
		FROM return_requests rr
		JOIN orders o ON o.id = rr.order_id
		LEFT JOIN payment_refunds pr ON pr.order_id = o.id
		WHERE rr.user_id = $1
		ORDER BY rr.created_at DESC, rr.id DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	returns := make([]viewmodels.Return, 0)
	for rows.Next() {
		var item viewmodels.Return
		var cents int64
		if err := rows.Scan(&item.ID, &item.Status, &item.Reason, &item.Requested, &item.ProductName, &item.ImageURL, &item.OrderNumber, &cents); err != nil {
			return nil, err
		}
		item.Amount = storefrontMoney(cents)
		returns = append(returns, item)
	}
	return returns, rows.Err()
}

func (h *LiveStorefrontHandler) customerPaymentActivity(ctx context.Context, userID int64) ([]viewmodels.PaymentActivity, error) {
	rows, err := h.pool.Query(ctx, `
		SELECT COALESCE(NULLIF(pa.provider_payment_id, ''), NULLIF(pa.provider_order_id, ''), o.order_number),
		       pa.amount_cents, pa.currency, pa.provider, o.order_number, pa.status, pa.created_at
		FROM payment_attempts pa
		JOIN orders o ON o.id = pa.order_id
		WHERE o.user_id = $1
		ORDER BY pa.created_at DESC, pa.id DESC
		LIMIT 20`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	activities := make([]viewmodels.PaymentActivity, 0)
	for rows.Next() {
		var item viewmodels.PaymentActivity
		var cents int64
		var currency, provider, status string
		var createdAt time.Time
		if err := rows.Scan(&item.Reference, &cents, &currency, &provider, &item.OrderNumber, &status, &createdAt); err != nil {
			return nil, err
		}
		item.Amount = storefrontMoney(cents)
		item.Date = createdAt.Format("2 Jan 2006")
		item.Method = strings.ToUpper(provider) + " · " + currency
		item.Status = strings.ToUpper(status[:1]) + strings.ReplaceAll(status[1:], "_", " ")
		activities = append(activities, item)
	}
	return activities, rows.Err()
}

func displayOrderStatus(status string) string {
	switch status {
	case "pending_payment":
		return "Awaiting payment"
	case "payment_failed":
		return "Payment failed"
	case "paid":
		return "Confirmed"
	case "processing":
		return "Processing"
	case "shipped":
		return "Shipped"
	case "delivered":
		return "Delivered"
	case "cancelled":
		return "Cancelled"
	case "refunded":
		return "Refunded"
	default:
		return "Order received"
	}
}

func displayPaymentStatus(paymentStatus, orderStatus string) string {
	if paymentStatus != "" {
		return strings.ToUpper(paymentStatus[:1]) + strings.ReplaceAll(paymentStatus[1:], "_", " ")
	}
	switch orderStatus {
	case "pending_payment":
		return "Payment pending"
	case "payment_failed":
		return "Failed"
	case "refunded":
		return "Refunded"
	case "cancelled":
		return "Cancelled"
	default:
		return "Paid"
	}
}

func (h *LiveStorefrontHandler) products(ctx context.Context, path, query, category, sortMode string) ([]viewmodels.Product, error) {
	var source []domaincatalog.Product
	var err error
	if path == "/search" || query != "" || category != "" || sortMode != "" {
		searchSort := map[string]string{"price-asc": applicationcatalog.SearchSortPriceAsc, "price-desc": applicationcatalog.SearchSortPriceDesc, "newest": applicationcatalog.SearchSortNewest}[sortMode]
		source, _, err = h.search.SearchWithOptions(ctx, query, categoryForSearch(category), searchSort, 48, 0)
	} else {
		source, err = h.catalog.List(ctx, "", 48)
	}
	if err != nil {
		return nil, err
	}
	result := make([]viewmodels.Product, 0, len(source))
	sellerNames := h.sellerNames(ctx, source)
	for _, product := range source {
		if category == "offers" && !product.HasCompareAt {
			continue
		}
		view := liveProduct(product, "")
		if seller := sellerNames[product.Slug]; seller != "" {
			view.Seller = seller
			view.SellerSlug = liveSlug(seller)
		}
		result = append(result, view)
	}
	return result, nil
}

func categoryForSearch(category string) string {
	switch strings.ToLower(strings.TrimSpace(category)) {
	case "offers":
		return ""
	case "sarees":
		return "textiles"
	default:
		return category
	}
}

func (h *LiveStorefrontHandler) searchSuggestions(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	products, err := h.products(r.Context(), "/search", query, "", "")
	if err != nil {
		h.renderError(w, http.StatusServiceUnavailable, "Search is temporarily unavailable.")
		return
	}
	page := viewmodels.CustomerPage{Query: query, Products: products}
	h.render(w, r, pages.SearchSuggestions(page.Products, query))
}

func (h *LiveStorefrontHandler) sellerFollowed(ctx context.Context, userID, sellerID int64) bool {
	if h.pool == nil || userID <= 0 || sellerID <= 0 {
		return false
	}
	var following bool
	if err := h.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM seller_follows WHERE user_id=$1 AND seller_id=$2)`, userID, sellerID).Scan(&following); err != nil {
		return false
	}
	return following
}

func (h *LiveStorefrontHandler) sellerFollowMutation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	userID, csrf, authenticated := h.session(r)
	slug := strings.Trim(strings.TrimPrefix(r.URL.Path, "/ui/seller-follow/"), "/")
	if !authenticated {
		next := "/makers/" + url.PathEscape(slug)
		http.Redirect(w, r, "/login?next="+url.QueryEscape(next), http.StatusSeeOther)
		return
	}
	if !h.customerRole(userID, r) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if subtle.ConstantTimeCompare([]byte(csrf), []byte(r.FormValue("csrf_token"))) != 1 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	var sellerID int64
	for _, seller := range h.sellers(r.Context()) {
		if seller.Slug == slug {
			sellerID = seller.ID
			break
		}
	}
	if sellerID == 0 {
		http.NotFound(w, r)
		return
	}
	switch r.FormValue("intent") {
	case "follow":
		_, err := h.pool.Exec(r.Context(), `INSERT INTO seller_follows (user_id,seller_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`, userID, sellerID)
		if err != nil {
			http.Error(w, "seller follow could not be saved", http.StatusServiceUnavailable)
			return
		}
	case "unfollow":
		if _, err := h.pool.Exec(r.Context(), `DELETE FROM seller_follows WHERE user_id=$1 AND seller_id=$2`, userID, sellerID); err != nil {
			http.Error(w, "seller follow could not be removed", http.StatusServiceUnavailable)
			return
		}
	default:
		http.Error(w, "invalid seller follow action", http.StatusUnprocessableEntity)
		return
	}
	http.Redirect(w, r, "/makers/"+url.PathEscape(slug), http.StatusSeeOther)
}

func (h *LiveStorefrontHandler) uiMutation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	userID, csrf, ok := h.session(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if subtle.ConstantTimeCompare([]byte(csrf), []byte(r.Header.Get("X-CSRF-Token"))) != 1 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	slug := strings.TrimPrefix(strings.TrimPrefix(r.URL.Path, "/ui/cart/"), "/ui/wishlist/")
	var productID, variantID int64
	requestedVariantID := int64(0)
	if raw := r.FormValue("variant_id"); raw != "" {
		requestedVariantID, _ = strconv.ParseInt(raw, 10, 64)
		if requestedVariantID <= 0 {
			http.Error(w, "invalid product option", http.StatusUnprocessableEntity)
			return
		}
	}
	variantQuery := `SELECT p.id,v.id FROM products p JOIN product_variants v ON v.product_id=p.id AND v.status='active' LEFT JOIN inventory_stock i ON i.variant_id=v.id WHERE p.slug=$1 AND p.status='approved' ORDER BY (COALESCE(i.available_quantity,0)>0) DESC,v.id LIMIT 1`
	variantArgs := []any{slug}
	if requestedVariantID > 0 {
		variantQuery = `SELECT p.id,v.id FROM products p JOIN product_variants v ON v.product_id=p.id AND v.status='active' WHERE p.slug=$1 AND p.status='approved' AND v.id=$2`
		variantArgs = append(variantArgs, requestedVariantID)
	}
	if err := h.pool.QueryRow(r.Context(), variantQuery, variantArgs...).Scan(&productID, &variantID); err != nil {
		if requestedVariantID > 0 {
			http.Error(w, "invalid product option", http.StatusUnprocessableEntity)
			return
		}
		http.NotFound(w, r)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/ui/wishlist/") {
		source := r.FormValue("source")
		if source == "cart" {
			if _, err := h.cart.MoveToWishlist(r.Context(), userID, variantID, productID); err != nil {
				http.Error(w, "wishlist update failed", http.StatusUnprocessableEntity)
				return
			}
			page, _ := h.page(r)
			h.render(w, r, pages.CartUpdate(page))
			return
		}
		if source == "wishlist" {
			if err := h.cart.RemoveWishlist(r.Context(), userID, productID); err != nil {
				http.Error(w, "wishlist update failed", http.StatusUnprocessableEntity)
				return
			}
			page, _ := h.page(r)
			h.render(w, r, pages.WishlistGridMutation(page))
			return
		}
		if err := h.cart.AddWishlist(r.Context(), userID, productID); err != nil {
			http.Error(w, "wishlist update failed", http.StatusUnprocessableEntity)
			return
		}
		page, _ := h.page(r)
		product, _ := h.catalog.Get(r.Context(), slug)
		h.render(w, r, components.WishlistMutation(page, liveProduct(product, csrf), false))
		return
	}
	quantity := int64(1)
	if raw := r.FormValue("quantity"); raw != "" {
		quantity, _ = strconv.ParseInt(raw, 10, 64)
	}
	if raw := r.FormValue("delta"); raw != "" {
		delta, _ := strconv.ParseInt(raw, 10, 64)
		cart, cartErr := h.cart.GetOrCreate(r.Context(), userID, "")
		if cartErr == nil {
			quantity = 1 + delta
			for _, item := range cart.Items {
				if item.VariantID == variantID {
					quantity = int64(item.Quantity) + delta
				}
			}
		}
	}
	var err error
	if quantity <= 0 {
		_, err = h.cart.Remove(r.Context(), userID, "", variantID)
	} else if r.FormValue("delta") != "" {
		_, err = h.cart.Update(r.Context(), userID, "", variantID, quantity)
	} else {
		_, err = h.cart.Add(r.Context(), userID, "", variantID, quantity)
	}
	if err != nil {
		http.Error(w, "cart update failed", http.StatusUnprocessableEntity)
		return
	}
	if r.FormValue("intent") == "buy" {
		w.Header().Set("HX-Redirect", "/checkout")
		return
	}
	page, _ := h.page(r)
	h.render(w, r, components.CartMutation(page))
}

func (h *LiveStorefrontHandler) liveCart(ctx context.Context, userID int64, csrf string) ([]viewmodels.CartItem, int) {
	cart, err := h.cart.GetOrCreate(ctx, userID, "")
	if err != nil {
		return nil, 0
	}
	items := make([]viewmodels.CartItem, 0, len(cart.Items))
	for _, item := range cart.Items {
		product, err := h.catalog.Get(ctx, item.ProductSlug)
		if err != nil {
			continue
		}
		productView := liveProduct(product, csrf)
		productView.Seller = h.sellerName(ctx, product.Slug)
		productView.VariantID = item.VariantID
		productView.SKU = item.SKU
		productView.PriceCents = item.UnitPriceCents
		productView.Price = storefrontMoney(item.UnitPriceCents)
		productView.Available = item.Available
		productView.InStock = item.Available > 0
		productView.Badge = map[bool]string{true: "In stock", false: "Sold out"}[productView.InStock]
		items = append(items, viewmodels.CartItem{Product: productView, VariantName: item.VariantName, Quantity: item.Quantity, Delivery: "4 - 6 days"})
	}
	return items, cart.ItemCount
}

func (h *LiveStorefrontHandler) liveWishlist(ctx context.Context, userID int64, csrf string) ([]viewmodels.Product, int) {
	items, err := h.cart.ListWishlist(ctx, userID)
	if err != nil {
		return nil, 0
	}
	result := make([]viewmodels.Product, 0, len(items))
	for _, item := range items {
		product, err := h.catalog.Get(ctx, item.Slug)
		if err == nil {
			productView := liveProduct(product, csrf)
			productView.Seller = h.sellerName(ctx, product.Slug)
			result = append(result, productView)
		}
	}
	return result, len(result)
}

func (h *LiveStorefrontHandler) session(r *http.Request) (int64, string, bool) {
	if h.sessions == nil {
		return 0, "", false
	}
	cookie, err := r.Cookie("wecratfs_session")
	if err != nil {
		return 0, "", false
	}
	record, err := h.sessions.Get(r.Context(), cookie.Value)
	if err != nil {
		return 0, "", false
	}
	return record.UserID, record.CSRFToken, true
}

func (h *LiveStorefrontHandler) customerRole(userID int64, r *http.Request) bool {
	if h.pool == nil || userID <= 0 {
		return false
	}
	var allowed bool
	err := h.pool.QueryRow(r.Context(), `SELECT EXISTS (SELECT 1 FROM user_roles WHERE user_id=$1 AND role_slug='customer')`, userID).Scan(&allowed)
	return err == nil && allowed
}

func (h *LiveStorefrontHandler) userRoles(r *http.Request, userID int64) []domainidentity.Role {
	if h.pool == nil || userID <= 0 {
		return nil
	}
	rows, err := h.pool.Query(r.Context(), `SELECT role_slug FROM user_roles WHERE user_id=$1 ORDER BY role_slug`, userID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var roles []domainidentity.Role
	for rows.Next() {
		var role string
		if rows.Scan(&role) == nil {
			roles = append(roles, domainidentity.Role(role))
		}
	}
	return roles
}

func (h *LiveStorefrontHandler) categories(ctx context.Context) []viewmodels.Category {
	rows, err := h.pool.Query(ctx, `SELECT c.slug,c.name,COUNT(p.id),COALESCE((SELECT image_url FROM products WHERE category_slug=c.slug ORDER BY id LIMIT 1),'') FROM categories c LEFT JOIN products p ON p.category_slug=c.slug AND p.status='approved' GROUP BY c.slug,c.name ORDER BY c.name`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	result := make([]viewmodels.Category, 0)
	for rows.Next() {
		var item viewmodels.Category
		if rows.Scan(&item.Slug, &item.Name, &item.Products, &item.ImageURL) == nil {
			item.Description = "Indian craft, home, textile and wellness pieces"
			result = append(result, item)
		}
	}
	return result
}

func (h *LiveStorefrontHandler) sellers(ctx context.Context) []viewmodels.Seller {
	rows, err := h.pool.Query(ctx, `SELECT s.id,s.display_name,s.status,
		(SELECT COUNT(*) FROM products p WHERE p.seller_id=s.id AND p.status='approved'),
		COALESCE((SELECT ROUND(AVG(pr.rating),1)::text FROM product_reviews pr JOIN products rp ON rp.id=pr.product_id WHERE rp.seller_id=s.id AND rp.status='approved' AND pr.status='published'),''),
		COALESCE((SELECT image_url FROM products WHERE seller_id=s.id AND status='approved' ORDER BY id LIMIT 1),'')
		FROM sellers s WHERE s.status='active' ORDER BY s.display_name`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	result := make([]viewmodels.Seller, 0)
	for rows.Next() {
		var id int64
		var name, status, rating, image string
		var count int
		if rows.Scan(&id, &name, &status, &count, &rating, &image) == nil {
			result = append(result, viewmodels.Seller{ID: id, Slug: liveSlug(name), Name: name, Status: status, Rating: rating, ImageURL: image, Products: count})
		}
	}
	return result
}

func (h *LiveStorefrontHandler) brands(ctx context.Context) []viewmodels.Brand {
	rows, err := h.pool.Query(ctx, `SELECT b.slug,b.name,COUNT(p.id),COALESCE((SELECT image_url FROM products WHERE brand_slug=b.slug AND status='approved' ORDER BY id LIMIT 1),'') FROM brands b LEFT JOIN products p ON p.brand_slug=b.slug AND p.status='approved' GROUP BY b.slug,b.name ORDER BY b.name`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	result := make([]viewmodels.Brand, 0)
	for rows.Next() {
		var item viewmodels.Brand
		if rows.Scan(&item.Slug, &item.Name, &item.Products, &item.ImageURL) == nil {
			item.Tagline = "Made by independent Indian makers"
			item.Description = "A live maker brand on WeeVCrafts."
			item.Location = "India"
			result = append(result, item)
		}
	}
	return result
}

func (h *LiveStorefrontHandler) sellerNames(ctx context.Context, products []domaincatalog.Product) map[string]string {
	result := make(map[string]string, len(products))
	if len(products) == 0 {
		return result
	}
	slugs := make([]string, 0, len(products))
	for _, product := range products {
		slugs = append(slugs, product.Slug)
	}
	rows, err := h.pool.Query(ctx, `SELECT p.slug,s.display_name FROM products p LEFT JOIN sellers s ON s.id=p.seller_id WHERE p.slug=ANY($1::text[])`, slugs)
	if err != nil {
		return result
	}
	defer rows.Close()
	for rows.Next() {
		var slug string
		var seller *string
		if rows.Scan(&slug, &seller) == nil && seller != nil {
			result[slug] = *seller
		}
	}
	return result
}

func (h *LiveStorefrontHandler) sellerName(ctx context.Context, slug string) string {
	var seller string
	_ = h.pool.QueryRow(ctx, `SELECT COALESCE(s.display_name,'') FROM products p LEFT JOIN sellers s ON s.id=p.seller_id WHERE p.slug=$1`, slug).Scan(&seller)
	return seller
}

func (h *LiveStorefrontHandler) productsForBrand(ctx context.Context, slug, csrf string) ([]viewmodels.Product, error) {
	var brandSlug string
	if err := h.pool.QueryRow(ctx, `SELECT slug FROM brands WHERE slug=$1 OR lower(name)=lower($1) LIMIT 1`, slug).Scan(&brandSlug); err != nil {
		return []viewmodels.Product{}, nil
	}
	rows, err := h.pool.Query(ctx, `SELECT slug FROM products WHERE brand_slug=$1 AND status='approved' ORDER BY featured_rank,id LIMIT 48`, brandSlug)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []viewmodels.Product
	for rows.Next() {
		var productSlug string
		if err := rows.Scan(&productSlug); err != nil {
			return nil, err
		}
		product, err := h.catalog.Get(ctx, productSlug)
		if err != nil {
			continue
		}
		view := liveProduct(product, csrf)
		view.Seller = h.sellerName(ctx, productSlug)
		view.SellerSlug = liveSlug(view.Seller)
		result = append(result, view)
	}
	return result, rows.Err()
}

func (h *LiveStorefrontHandler) productsForSeller(ctx context.Context, slug, csrf, sortMode string) ([]viewmodels.Product, error) {
	rows, err := h.pool.Query(ctx, `SELECT p.slug,s.display_name FROM products p JOIN sellers s ON s.id=p.seller_id
		LEFT JOIN LATERAL (SELECT pv.price_cents FROM product_variants pv WHERE pv.product_id=p.id AND pv.status='active' ORDER BY pv.id LIMIT 1) variant ON TRUE
		WHERE p.status='approved' AND s.status='active'
		ORDER BY CASE WHEN $1='newest' THEN p.created_at END DESC NULLS LAST,
		         CASE WHEN $1='price-asc' THEN variant.price_cents END ASC NULLS LAST,
		         CASE WHEN $1='price-desc' THEN variant.price_cents END DESC NULLS LAST,
		         p.featured_rank,p.id`, sortMode)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var slugs []string
	for rows.Next() {
		var productSlug, seller string
		if err := rows.Scan(&productSlug, &seller); err != nil {
			return nil, err
		}
		if liveSlug(seller) == slug {
			slugs = append(slugs, productSlug)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	result := make([]viewmodels.Product, 0, len(slugs))
	for _, productSlug := range slugs {
		product, err := h.catalog.Get(ctx, productSlug)
		if err != nil {
			continue
		}
		view := liveProduct(product, csrf)
		view.Seller = h.sellerName(ctx, productSlug)
		view.SellerSlug = liveSlug(view.Seller)
		result = append(result, view)
	}
	return result, nil
}

func (h *LiveStorefrontHandler) render(w http.ResponseWriter, r *http.Request, component templ.Component) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := component.Render(r.Context(), w); err != nil {
		h.renderError(w, http.StatusInternalServerError, "The page could not be rendered.")
	}
}

func (h *LiveStorefrontHandler) renderError(w http.ResponseWriter, status int, message string) {
	http.Error(w, message, status)
}

func liveProduct(product domaincatalog.Product, csrf string) viewmodels.Product {
	price := storefrontMoney(product.PriceCents)
	compare := ""
	if product.HasCompareAt && product.CompareAtCents > product.PriceCents {
		compare = storefrontMoney(product.CompareAtCents)
	}
	discount := ""
	if product.HasCompareAt && product.CompareAtCents > product.PriceCents {
		discount = fmt.Sprintf("%d%% OFF", (product.CompareAtCents-product.PriceCents)*100/product.CompareAtCents)
	}
	rating := ""
	if product.Rating > 0 {
		rating = strconv.FormatFloat(product.Rating, 'f', 1, 64)
	}
	return viewmodels.Product{ID: product.ID, VariantID: product.VariantID, PriceCents: product.PriceCents, Slug: product.Slug, Name: product.Name, Description: product.Description, DeliveryLabel: product.DeliveryLabel, Seller: product.Brand, BrandName: product.Brand, Location: "India", Category: product.CategoryName, ImageURL: product.ImageURL, ImageSrcSet: imageSrcSet(product.ImageURL), Badge: map[bool]string{true: "In stock", false: "Sold out"}[product.Available > 0], Price: price, CompareAt: compare, Discount: discount, Rating: rating, Reviews: product.ReviewCount, InStock: product.Available > 0, CSRFToken: csrf}
}

func (h *LiveStorefrontHandler) customerProductVariants(r *http.Request, slug string) ([]viewmodels.ProductVariant, error) {
	rows, err := h.pool.Query(r.Context(), `SELECT pv.id,pv.name,pv.sku,pv.price_cents,COALESCE(pv.compare_at_cents,0),
		pv.compare_at_cents IS NOT NULL,COALESCE(i.available_quantity,0),COALESCE(i.reserved_quantity,0),pv.status
		FROM products p JOIN product_variants pv ON pv.product_id=p.id AND pv.status='active'
		LEFT JOIN inventory_stock i ON i.variant_id=pv.id
		WHERE p.slug=$1 AND p.status='approved' ORDER BY (COALESCE(i.available_quantity,0)>0) DESC,pv.id`, slug)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	variants := make([]viewmodels.ProductVariant, 0)
	for rows.Next() {
		var id, price, compareAt, available, reserved int64
		var name, sku, status string
		var hasCompare bool
		if err := rows.Scan(&id, &name, &sku, &price, &compareAt, &hasCompare, &available, &reserved, &status); err != nil {
			return nil, err
		}
		view := viewmodels.ProductVariant{ID: strconv.FormatInt(id, 10), Name: name, SKU: sku, PriceCents: strconv.FormatInt(price, 10),
			Available: strconv.FormatInt(available, 10), Reserved: strconv.FormatInt(reserved, 10), Status: status,
			Price: storefrontMoney(price), InStock: available > 0, StockMessage: "Currently unavailable"}
		if view.InStock {
			view.StockMessage = "In stock"
		}
		view.HasCompareAt = hasCompare && compareAt > price
		if view.HasCompareAt {
			view.CompareAtCents = strconv.FormatInt(compareAt, 10)
			view.CompareAt = storefrontMoney(compareAt)
			view.Discount = fmt.Sprintf("%d%% OFF", (compareAt-price)*100/compareAt)
		}
		variants = append(variants, view)
	}
	return variants, rows.Err()
}

func liveTotals(items []viewmodels.CartItem) (string, string, string, string) {
	var subtotal int64
	for _, item := range items {
		subtotal += item.Product.PriceCents * int64(item.Quantity)
	}
	shipping := int64(0)
	if subtotal > 0 && subtotal < 99900 {
		shipping = 9900
	}
	return storefrontMoney(subtotal), storefrontMoney(shipping), storefrontMoney(0), storefrontMoney(subtotal + shipping)
}

func cents(value string) int64 {
	value = strings.NewReplacer("INR", "", ",", "", "₹", "", " ", "").Replace(value)
	parsed, _ := strconv.ParseFloat(value, 64)
	return int64(parsed * 100)
}

func storefrontMoney(value int64) string { return fmt.Sprintf("INR %d", value/100) }

func imageSrcSet(value string) string {
	if !strings.HasSuffix(value, ".webp") {
		return ""
	}
	return strings.TrimSuffix(value, ".webp") + "-480.webp 480w, " + value + " 960w"
}

func liveTitle(path string) string {
	switch {
	case path == "/":
		return "Handmade Today"
	case path == "/login":
		return "Sign in"
	case path == "/register":
		return "Create your account"
	case path == "/forgot-password":
		return "Reset your password"
	case path == "/reset-password":
		return "Choose a new password"
	case path == "/account":
		return "My Account"
	case path == "/orders":
		return "My Orders"
	case path == "/cart":
		return "Your Cart"
	case path == "/wishlist":
		return "My Wishlist"
	case path == "/checkout":
		return "Checkout"
	case path == "/tracking":
		return "Track your order"
	case path == "/returns":
		return "Returns and refunds"
	case path == "/payments":
		return "Payment methods"
	case path == "/account/profile":
		return "Profile information"
	case path == "/account/addresses":
		return "Saved addresses"
	case path == "/account/notifications":
		return "Notification preferences"
	case path == "/account/security":
		return "Security and sessions"
	case path == "/account/privacy":
		return "Privacy preferences"
	case path == "/account/reviews":
		return "Reviews"
	case path == "/faq":
		return "FAQs"
	case path == "/contact":
		return "Contact us"
	case path == "/shipping":
		return "Shipping information"
	case path == "/size-guide":
		return "Size guide"
	case path == "/terms":
		return "Terms and conditions"
	case path == "/privacy":
		return "Privacy policy"
	case path == "/about", path == "/our-story":
		return "Our story"
	case path == "/sustainability":
		return "Sustainability"
	case path == "/press":
		return "Press"
	case path == "/careers":
		return "Careers"
	case strings.HasPrefix(path, "/orders/"):
		return "Order details"
	case path == "/categories":
		return "Categories"
	case path == "/brands":
		return "Brands"
	case path == "/deals":
		return "Offers"
	case strings.HasPrefix(path, "/products/"):
		return "Product"
	default:
		return "Products"
	}
}

func liveSlug(value string) string {
	return strings.Trim(strings.NewReplacer("&", "and", "'", "", " ", "-").Replace(strings.ToLower(value)), "-")
}

func liveBrand(path string, brands []viewmodels.Brand) viewmodels.Brand {
	slug, _ := url.PathUnescape(strings.TrimPrefix(path, "/brands/"))
	for _, brand := range brands {
		if brand.Slug == slug {
			return brand
		}
	}
	return viewmodels.Brand{Slug: slug, Name: slug}
}

func liveSeller(path string, sellers []viewmodels.Seller) viewmodels.Seller {
	slug, _ := url.PathUnescape(strings.TrimPrefix(path, "/makers/"))
	for _, seller := range sellers {
		if seller.Slug == slug {
			return seller
		}
	}
	return viewmodels.Seller{Slug: slug, Name: slug}
}
