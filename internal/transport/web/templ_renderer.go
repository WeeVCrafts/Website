package web

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/a-h/templ"

	domaincatalog "github.com/wecratfs/commerce/internal/domain/catalog"
	domaincommerce "github.com/wecratfs/commerce/internal/domain/commerce"
	"github.com/wecratfs/commerce/internal/web/viewmodels"
	"github.com/wecratfs/commerce/web/legacy"
	"github.com/wecratfs/commerce/web/pages"
)

func (h *Handler) renderTempl(w http.ResponseWriter, r *http.Request, status int, name string, data pageData) {
	component := h.templComponent(name, data)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := component.Render(r.Context(), w); err != nil {
		return
	}
}

func (h *Handler) templComponent(name string, data pageData) templ.Component {
	page := legacyCustomerPage(name, data)
	switch name {
	case "home":
		return pages.Home(page)
	case "product":
		return pages.ProductDetail(page)
	case "search", "deals":
		if name == "deals" {
			return pages.Deals(page)
		}
		return pages.Listing(page)
	case "brands":
		return pages.Brands(page)
	case "brand":
		return pages.BrandDetail(page, legacyBrand(data))
	case "login", "register", "forgot-password", "reset-password", "verify-email":
		return pages.Auth(page)
	case "cart":
		return pages.Cart(page)
	case "checkout":
		return pages.Checkout(page)
	case "wishlist":
		return pages.Wishlist(page)
	case "orders":
		return pages.Orders(page)
	case "order":
		if page.SelectedOrder.Number == "" {
			return pages.Empty(page)
		}
		return pages.OrderDetail(page)
	case "tracking":
		return pages.Tracking(page)
	case "payment-status":
		return pages.PaymentStatus(page, legacyPaymentState(data))
	case "privacy", "privacy-center":
		if name == "privacy-center" {
			return pages.PrivacyCenter(page)
		}
		return pages.InfoPage(page)
	case "policy":
		return legacy.Document(legacyPage(name, data))
	case "cart-add-response":
		return legacy.CartAddResponse(page, legacyNotice(data, "Added to your cart."))
	case "cart-mutation":
		return legacy.CartMutation(page, legacyNotice(data, "Cart updated"))
	case "wishlist-add-response":
		return legacy.WishlistAddResponse(page, legacyNotice(data, "Saved to your wishlist."))
	case "wishlist-mutation":
		return legacy.WishlistMutation(page, legacyNotice(data, "Wishlist updated"))
	case "search-suggestions":
		return pages.SearchSuggestions(page.Products, page.Query)
	default:
		return legacy.Document(legacyPage(name, data))
	}
}

func legacyPage(name string, data pageData) viewmodels.LegacyPage {
	title := strings.TrimSpace(data.Title)
	if title == "" {
		title = strings.TrimSpace(name)
	}
	return viewmodels.LegacyPage{
		Mode: name, Brand: data.Brand, Tagline: data.Tagline, Description: data.Description,
		Title: title, Error: data.Error, Notice: data.Notice, FormEmail: data.FormEmail,
		CSRFToken: data.CSRFToken, Customer: legacyCustomerPage(name, data),
		Products: legacyProducts(data.Products), Product: legacyProduct(data.Product),
		Lines:          legacyLines(name, data),
		CartCount:      data.Cart.ItemCount,
		ManagedProduct: legacyManagedProduct(data.ManagedProduct), Staff: legacyStaff(data.StaffMembers),
	}
}

func legacyManagedProduct(product domaincommerce.ManagedProduct) viewmodels.LegacyProductDraft {
	return viewmodels.LegacyProductDraft{ID: product.ID, Name: product.Name, Description: product.Description, SKU: product.SKU, DeliveryLabel: product.DeliveryLabel, PriceCents: product.PriceCents, CompareAtCents: product.CompareAtCents}
}

func legacyStaff(items []domaincommerce.SellerStaffMember) []viewmodels.LegacyStaffMember {
	result := make([]viewmodels.LegacyStaffMember, 0, len(items))
	for _, item := range items {
		result = append(result, viewmodels.LegacyStaffMember{Email: item.Email, DisplayName: item.DisplayName, Status: item.Status, Permissions: item.Permissions})
	}
	return result
}

func legacyCustomerPage(name string, data pageData) viewmodels.CustomerPage {
	route := legacyRoute(name)
	products := legacyProducts(data.Products)
	if len(products) == 0 && data.Product.Name != "" {
		products = []viewmodels.Product{legacyProduct(data.Product)}
	}
	category := data.Category
	if category == "" {
		category = data.SearchCategory
	}
	pages := data.SearchPage
	if pages == 0 && (data.SearchQuery != "" || data.SearchCategory != "") {
		pages = 1
	}
	if data.SearchHasNext {
		pages++
	}
	page := viewmodels.CustomerPage{
		Route: route, Title: data.Title, Query: data.SearchQuery, Category: category,
		Sort: data.SearchSort, Page: data.SearchPage, Pages: pages, Brand: data.Brand, Tagline: data.Tagline, Description: data.Description,
		CSRFToken: data.CSRFToken, Authenticated: data.Authenticated, Notice: data.Notice,
		AccountUserID: data.AccountUserID,
		Products:      products, Product: legacyProduct(data.Product), Cart: legacyCartItems(data.Cart),
		CartCount: data.Cart.ItemCount, SavedProducts: legacyWishlist(data.Wishlist),
		WishlistCount: len(data.Wishlist), Orders: legacyOrders(data.Orders),
		PrivacyCenter: data.PrivacyCenter,
		Brands:        legacyBrands(data.Brands),
		Filters:       url.Values{"q": {data.SearchQuery}, "category": {data.SearchCategory}, "sort": {data.SearchSort}},
	}
	// This transport receives a selected filter but no category directory.
	// Keep it as a removable checkbox; the live storefront provides the full directory.
	if category != "" && category != "offers" {
		page.Categories = []viewmodels.Category{{Slug: category, Name: category}}
	}
	if page.CartCount == 0 {
		for _, item := range page.Cart {
			page.CartCount += item.Quantity
		}
	}
	page.ItemsTotal = legacyMoney(data.Cart.SubtotalCents)
	page.ShippingTotal = legacyMoney(0)
	page.DiscountTotal = legacyMoney(0)
	page.Total = page.ItemsTotal
	if data.HasProduct {
		page.Product = legacyProduct(data.Product)
	}
	if data.OrderDetail.OrderNumber != "" {
		page.SelectedOrder = legacyOrderDetail(data.OrderDetail)
	}
	if name == "payment-status" && strings.EqualFold(strings.TrimSpace(data.OrderDetail.PaymentStatus), "captured") {
		page.Notice = "Payment confirmed"
	}
	if name == "tracking" && len(data.OrderDetail.Fulfillments) > 0 {
		page.Notice = strings.TrimSpace(data.OrderDetail.Fulfillments[0].TrackingNumber)
	}
	return page
}

func legacyRoute(name string) string {
	switch name {
	case "home":
		return "/"
	case "product":
		return "/products/legacy"
	case "search":
		return "/search"
	case "deals":
		return "/deals"
	case "brands":
		return "/brands"
	case "brand":
		return "/brands/legacy"
	case "cart", "cart-add-response", "cart-mutation":
		return "/cart"
	case "checkout":
		return "/checkout"
	case "wishlist", "wishlist-add-response", "wishlist-mutation":
		return "/wishlist"
	case "orders", "order":
		return "/orders"
	case "tracking":
		return "/tracking"
	case "payment-status":
		return "/payments/success"
	case "login", "register", "forgot-password", "reset-password", "verify-email":
		return "/" + name
	case "privacy", "policy", "privacy-center":
		if name == "privacy-center" {
			return "/privacy/center"
		}
		return "/privacy"
	default:
		return "/" + strings.TrimPrefix(name, "/")
	}
}

func legacyProduct(product domaincatalog.Product) viewmodels.Product {
	price := legacyMoney(product.PriceCents)
	compare := ""
	if product.HasCompareAt && product.CompareAtCents > product.PriceCents {
		compare = legacyMoney(product.CompareAtCents)
	}
	rating := ""
	if product.Rating > 0 {
		rating = strconv.FormatFloat(product.Rating, 'f', 1, 64)
	}
	return viewmodels.Product{
		ID: product.ID, VariantID: product.VariantID, SKU: product.SKU, Slug: product.Slug,
		Name: product.Name, Description: product.Description, DeliveryLabel: product.DeliveryLabel,
		Seller: product.Brand, BrandName: product.Brand, Category: product.CategoryName,
		ImageURL: product.ImageURL, ImageSrcSet: product.ImageURL, PriceCents: product.PriceCents,
		CompareAtCents: product.CompareAtCents, Price: price, CompareAt: compare, Rating: rating,
		Reviews: product.ReviewCount, Available: product.Available, InStock: product.Available > 0,
		Badge: map[bool]string{true: "In stock", false: "Sold out"}[product.Available > 0],
	}
}

func legacyProducts(products []domaincatalog.Product) []viewmodels.Product {
	result := make([]viewmodels.Product, 0, len(products))
	for _, product := range products {
		result = append(result, legacyProduct(product))
	}
	return result
}

func legacyCartItems(cart domaincommerce.Cart) []viewmodels.CartItem {
	result := make([]viewmodels.CartItem, 0, len(cart.Items))
	for _, item := range cart.Items {
		product := viewmodels.Product{ID: item.ProductID, VariantID: item.VariantID, Slug: item.ProductSlug, Name: item.ProductName, SKU: item.SKU, PriceCents: item.UnitPriceCents, Price: legacyMoney(item.UnitPriceCents), Seller: "Independent maker", Location: "India", Available: item.Available, InStock: item.Available > 0, Badge: map[bool]string{true: "In stock", false: "Sold out"}[item.Available > 0]}
		result = append(result, viewmodels.CartItem{Product: product, VariantName: item.VariantName, Quantity: item.Quantity, Delivery: "Delivery estimate unavailable"})
	}
	return result
}

func legacyWishlist(items []domaincommerce.WishlistItem) []viewmodels.Product {
	result := make([]viewmodels.Product, 0, len(items))
	for _, item := range items {
		result = append(result, viewmodels.Product{ID: item.ProductID, Slug: item.Slug, Name: item.Name, Seller: item.Brand, PriceCents: item.PriceCents, Price: legacyMoney(item.PriceCents), Available: item.Available, InStock: item.Available > 0, ImageURL: "/assets/images/mock/hero.webp"})
	}
	return result
}

func legacyOrders(items []domaincommerce.Order) []viewmodels.Order {
	result := make([]viewmodels.Order, 0, len(items))
	for _, item := range items {
		result = append(result, viewmodels.Order{Number: item.OrderNumber, Status: item.Status, State: item.Status, Date: item.CreatedAt.Format("2 Jan 2006"), Total: legacyMoney(item.TotalCents), Subtotal: legacyMoney(item.SubtotalCents), Shipping: legacyMoney(item.ShippingCents), Discount: legacyMoney(item.DiscountCents)})
	}
	return result
}

func legacyOrderDetail(detail domaincommerce.OrderDetail) viewmodels.Order {
	order := legacyOrders([]domaincommerce.Order{detail.Order})[0]
	for _, item := range detail.Items {
		order.Items = append(order.Items, viewmodels.CartItem{Product: viewmodels.Product{Name: item.ProductName, SKU: item.SKU, PriceCents: item.UnitPriceCents, Price: legacyMoney(item.UnitPriceCents), Seller: "Independent maker", ImageURL: "/assets/images/mock/hero.webp"}, Quantity: item.Quantity})
	}
	return order
}

func legacyBrand(data pageData) viewmodels.Brand {
	return viewmodels.Brand{Slug: data.BrandSlug, Name: data.BrandName, Tagline: data.Description, Description: data.Description, ImageURL: "/assets/images/mock/hero.webp"}
}

func legacyBrands(items []BrandSummary) []viewmodels.Brand {
	result := make([]viewmodels.Brand, 0, len(items))
	for _, item := range items {
		result = append(result, viewmodels.Brand{Slug: item.Slug, Name: item.Name, Products: item.ProductCount, ImageURL: "/assets/images/mock/hero.webp", Location: "India"})
	}
	return result
}

func legacyLines(name string, data pageData) []string {
	if name == "support" && data.SupportSeller {
		return []string{"Seller support"}
	}
	labels := map[string]string{
		"admin-sellers":    "Seller applications",
		"admin-audit":      "Audit log",
		"seller-audit":     "Seller activity",
		"roles":            "Role management",
		"admin-orders":     "Marketplace orders",
		"admin-brands":     "Brands and categories",
		"admin-customers":  "Customer directory",
		"finance":          "Payments and refunds",
		"seller-orders":    "Seller orders",
		"seller-analytics": "Seller analytics",
		"support":          "Support queue",
		"seller-support":   "Seller support",
		"shipping":         "Shipping details",
	}
	if label := labels[name]; label != "" {
		return []string{label}
	}
	return nil
}

func legacyMoney(cents int64) string {
	return fmt.Sprintf("INR %d.%02d", cents/100, cents%100)
}

func legacyNotice(data pageData, fallback string) string {
	if strings.TrimSpace(data.Notice) != "" {
		return data.Notice
	}
	return fallback
}

func legacyPaymentState(data pageData) string {
	if status := strings.ToLower(strings.TrimSpace(data.OrderDetail.PaymentStatus)); status != "" {
		if strings.Contains(status, "fail") {
			return "failed"
		}
		if status == "success" || status == "paid" || status == "captured" {
			return "success"
		}
	}
	value := strings.ToLower(strings.TrimSpace(data.PaymentIntent.Status))
	if strings.Contains(value, "fail") {
		return "failed"
	}
	if value == "success" || value == "paid" || value == "captured" {
		return "success"
	}
	return "pending"
}
