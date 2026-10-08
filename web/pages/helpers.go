package pages

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/wecratfs/commerce/internal/web/viewmodels"
)

func privacyDate(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format("02 Jan 2006")
}

func privacyGrantedValue(enabled bool) string {
	if enabled {
		return "false"
	}
	return "true"
}

func privacyEnabledLabel(enabled bool) string {
	if enabled {
		return "enabled"
	}
	return "disabled"
}

func privacyConsentStatus(granted bool) string {
	if granted {
		return "granted"
	}
	return "withdrawn"
}

func privacyConsentAction(enabled bool, purpose string) string {
	if enabled {
		if purpose == "marketing" {
			return "Withdraw marketing consent"
		}
		return "Withdraw analytics consent"
	}
	if purpose == "marketing" {
		return "Allow marketing email"
	}
	return "Allow optional analytics"
}

func privacyRequestID(value int64) string { return strconv.FormatInt(value, 10) }

func pageIsIndexable(p viewmodels.CustomerPage) bool {
	switch {
	case p.Route == "/", p.Route == "/categories", p.Route == "/brands", p.Route == "/products":
		return true
	case strings.HasPrefix(p.Route, "/products/"), strings.HasPrefix(p.Route, "/brands/"), strings.HasPrefix(p.Route, "/makers/"):
		return true
	default:
		return false
	}
}

func pageRobots(p viewmodels.CustomerPage) string {
	if pageIsIndexable(p) {
		return "index,follow"
	}
	return "noindex,nofollow"
}

func pageCanonicalURL(p viewmodels.CustomerPage) string {
	base := strings.TrimRight(strings.TrimSpace(p.PublicURL), "/")
	if base == "" {
		base = "http://localhost:8080"
	}
	path := p.Route
	if path == "" {
		path = "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return base + path
}

func pageOGTitle(p viewmodels.CustomerPage) string {
	if strings.TrimSpace(p.Title) != "" {
		return p.Title
	}
	return "Weevcrafts"
}

func pageOGDescription(p viewmodels.CustomerPage) string {
	if strings.TrimSpace(p.Product.Description) != "" && strings.HasPrefix(p.Route, "/products/") {
		return p.Product.Description
	}
	return p.Description
}

func pageOGType(p viewmodels.CustomerPage) string {
	if strings.HasPrefix(p.Route, "/products/") {
		return "product"
	}
	return "website"
}

func pageOGImage(p viewmodels.CustomerPage) string {
	if strings.TrimSpace(p.Product.ImageURL) != "" && strings.HasPrefix(p.Route, "/products/") {
		return absolutePageURL(p, p.Product.ImageURL)
	}
	return ""
}

func absolutePageURL(p viewmodels.CustomerPage, value string) string {
	value = strings.TrimSpace(value)
	if value == "" || strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") {
		return value
	}
	base := strings.TrimRight(strings.TrimSpace(p.PublicURL), "/")
	if base == "" {
		base = "http://localhost:8080"
	}
	return base + "/" + strings.TrimLeft(value, "/")
}

func productStructuredData(p viewmodels.CustomerPage) string {
	if !strings.HasPrefix(p.Route, "/products/") || strings.TrimSpace(p.Product.Name) == "" {
		return ""
	}
	availability := "https://schema.org/OutOfStock"
	if p.Product.InStock || p.Product.Available > 0 {
		availability = "https://schema.org/InStock"
	}
	data := map[string]any{
		"@context": "https://schema.org",
		"@type": "Product",
		"name": p.Product.Name,
		"description": pageOGDescription(p),
		"image": []string{absolutePageURL(p, p.Product.ImageURL)},
		"sku": p.Product.SKU,
		"offers": map[string]any{
			"@type": "Offer",
			"url": pageCanonicalURL(p),
			"priceCurrency": "INR",
			"price": fmt.Sprintf("%.2f", float64(p.Product.PriceCents)/100),
			"availability": availability,
		},
	}
	if strings.TrimSpace(p.Product.Seller) != "" {
		data["brand"] = map[string]any{"@type": "Brand", "name": p.Product.Seller}
	}
	if rating, err := strconv.ParseFloat(strings.TrimSpace(p.Product.Rating), 64); err == nil && rating > 0 && p.Product.Reviews > 0 {
		data["aggregateRating"] = map[string]any{
			"@type": "AggregateRating", "ratingValue": rating, "reviewCount": p.Product.Reviews,
		}
	}
	encoded, err := json.Marshal(data)
	if err != nil {
		return ""
	}
	return string(encoded)
}

func cartDeltaValues(variantID int64, delta int) string {
	return fmt.Sprintf(`{"variant_id":"%d","delta":"%d"}`, variantID, delta)
}

func cartQuantityValue(quantity int) string { return strconv.Itoa(quantity) }

func cartRemoveValues(variantID int64) string {
	return fmt.Sprintf(`{"variant_id":"%d","remove":"true"}`, variantID)
}

func customerSupportMessageClass(role string) string {
	if strings.EqualFold(strings.TrimSpace(role), "support agent") {
		return "customer-support-message customer-support-message--agent"
	}
	return "customer-support-message customer-support-message--customer"
}

func breadcrumbHref(item string) string {
	switch strings.ToLower(strings.TrimSpace(item)) {
	case "home":
		return "/"
	case "categories":
		return "/categories"
	case "brands":
		return "/brands"
	case "deals":
		return "/deals"
	case "search results":
		return "/products"
	case "sarees":
		return "/products?category=sarees"
	case "arts":
		return "/products?category=arts"
	case "crafts":
		return "/products?category=crafts"
	case "home & living":
		return "/products?category=home-living"
	case "fashion":
		return "/products?category=fashion"
	case "gifts":
		return "/products?category=gifts"
	case "my account":
		return "/account"
	case "account":
		return "/account"
	case "my orders":
		return "/orders"
	case "your cart":
		return "/cart"
	case "my wishlist":
		return "/wishlist"
	case "returns and refunds":
		return "/returns"
	case "checkout":
		return "/checkout"
	case "track order", "track your order":
		return "/tracking"
	case "payments", "payment methods":
		return "/payments"
	case "order details":
		return "/orders/WC2504267819"
	case "chanderi silk cotton saree":
		return "/products/chanderi-royal"
	default:
		return "/products"
	}
}

func authNext(filters url.Values) string {
	candidate := strings.TrimSpace(filters.Get("next"))
	if candidate == "" || !strings.HasPrefix(candidate, "/") || strings.HasPrefix(candidate, "//") || strings.Contains(candidate, "\\") {
		return ""
	}
	return candidate
}

func authToken(filters url.Values) string { return filters.Get("token") }

func customerInitials(name string) string {
	parts := strings.Fields(strings.TrimSpace(name))
	if len(parts) == 0 {
		return "W"
	}
	initials := []rune(strings.ToUpper(string([]rune(parts[0])[0])))
	if len(parts) > 1 {
		initials = append(initials, []rune(strings.ToUpper(string([]rune(parts[len(parts)-1])[0])))...)
	}
	return string(initials)
}

func liveAccountLinkClass(active, target string) string {
	if active == target {
		return "active"
	}
	return ""
}

func liveAccountCurrent(active, target string) string {
	if active == target {
		return "page"
	}
	return ""
}

func paymentMethodLabel(mode string) string {
	if strings.EqualFold(strings.TrimSpace(mode), "local") {
		return "Local test payment"
	}
	return "Razorpay secure checkout"
}

func breadcrumbCurrent(index, total int) string {
	if index == total-1 {
		return "page"
	}
	return ""
}

func paginationClass(page, current int) string {
	if page == current {
		return "active"
	}
	return ""
}

func paginationCurrent(page, current int) string {
	if page == current {
		return "page"
	}
	return ""
}

func pageLabel(page int) string { return strconv.Itoa(page) }

func listingResultCount(page viewmodels.CustomerPage) int {
	if page.TotalResults > 0 || len(page.Products) == 0 {
		return page.TotalResults
	}
	return len(page.Products)
}

func orderPath(number string) string {
	return "/orders/" + strings.TrimPrefix(strings.TrimSpace(number), "#")
}

func orderSubset(orders []viewmodels.Order, status string) []viewmodels.Order {
	status = strings.ToLower(strings.TrimSpace(status))
	if status == "" {
		return orders
	}
	filtered := make([]viewmodels.Order, 0, len(orders))
	for _, order := range orders {
		orderStatus := strings.ToLower(strings.TrimSpace(order.State))
		if orderStatus == "" {
			orderStatus = strings.ToLower(strings.TrimSpace(order.Status))
		}
		matches := (status == "active" && (orderStatus == "pending_payment" || orderStatus == "paid" || orderStatus == "confirmed" || orderStatus == "processing" || orderStatus == "packed" || orderStatus == "shipped")) ||
			(status == "delivered" && orderStatus == "delivered") ||
			(status == "cancelled" && orderStatus == "cancelled")
		if matches {
			filtered = append(filtered, order)
		}
	}
	return filtered
}

func orderCount(orders []viewmodels.Order, status string) int {
	return len(orderSubset(orders, status))
}

func orderItemCount(order viewmodels.Order) int {
	count := 0
	for _, item := range order.Items {
		count += item.Quantity
	}
	return count
}

func orderSeller(order viewmodels.Order) string {
	item, ok := firstCartItem(order.Items)
	if !ok || strings.TrimSpace(item.Product.Seller) == "" {
		return "Independent maker"
	}
	return item.Product.Seller
}

func orderDeliveryCopy(order viewmodels.Order) string {
	switch orderState(order) {
	case "pending_payment":
		return "Your order is awaiting payment."
	case "payment_failed":
		return "Payment was not completed."
	case "shipped":
		return "Your order has been shipped."
	case "refunded":
		return "Your order has been refunded."
	case "delivered":
		return "Delivered"
	case "cancelled":
		return "Order cancelled"
	default:
		return "Delivery status will update as the maker ships"
	}
}

func orderTabClass(filters url.Values, status string) string {
	if strings.EqualFold(filters.Get("status"), status) || (status == "" && filters.Get("status") == "") {
		return "active"
	}
	return ""
}

func orderTabCurrent(filters url.Values, status string) string {
	if orderTabClass(filters, status) == "active" {
		return "page"
	}
	return ""
}

func tabSelected(active bool) string {
	if active {
		return "true"
	}
	return "false"
}

func productSlice(products []viewmodels.Product, start, end int) []viewmodels.Product {
	if start < 0 {
		start = 0
	}
	if start > len(products) {
		start = len(products)
	}
	if end < start {
		end = start
	}
	if end > len(products) {
		end = len(products)
	}
	return products[start:end]
}

func firstCartItem(items []viewmodels.CartItem) (viewmodels.CartItem, bool) {
	if len(items) == 0 {
		return viewmodels.CartItem{}, false
	}
	return items[0], true
}

func firstCartItemValue(items []viewmodels.CartItem) viewmodels.CartItem {
	item, _ := firstCartItem(items)
	return item
}

func orderSlice(orders []viewmodels.Order, start int) []viewmodels.Order {
	if start < 0 {
		start = 0
	}
	if start > len(orders) {
		start = len(orders)
	}
	return orders[start:]
}

func firstOrderList(orders []viewmodels.Order) []viewmodels.Order {
	if len(orders) == 0 {
		return nil
	}
	return orders[:1]
}

func firstOrder(orders []viewmodels.Order) viewmodels.Order {
	if len(orders) == 0 {
		return viewmodels.Order{}
	}
	return orders[0]
}

func orderStatusTitle(order viewmodels.Order) string {
	switch orderState(order) {
	case "pending_payment":
		return "Your order is awaiting payment."
	case "payment_failed":
		return "Payment was not completed."
	case "shipped":
		return "Your order has been shipped."
	case "refunded":
		return "Your order has been refunded."
	case "delivered":
		return "Your order has arrived safely."
	case "cancelled":
		return "This order was cancelled."
	default:
		return "Your order is being prepared with care."
	}
}

func orderStatusCopy(order viewmodels.Order) string {
	switch orderState(order) {
	case "pending_payment":
		return "Your order is saved. Check payment status before placing another order."
	case "payment_failed":
		return "This order remains in your history. No shipment is confirmed; contact support if you need help."
	case "shipped":
		return "See the maker's shipment updates in order tracking."
	case "refunded":
		return "Check your payment history for the recorded refund status."
	case "delivered":
		return "We hope it brings something beautiful into your everyday."
	case "cancelled":
		return "If you need help with this cancellation, our customer care team is here."
	default:
		return "We will notify you as soon as it ships."
	}
}

func orderState(order viewmodels.Order) string {
	state := strings.TrimSpace(order.State)
	if state == "" {
		state = order.Status
	}
	return strings.ToLower(strings.TrimSpace(state))
}

func firstReturn(returns []viewmodels.Return) viewmodels.Return {
	if len(returns) == 0 {
		return viewmodels.Return{}
	}
	return returns[0]
}

func returnAt(returns []viewmodels.Return, index int) viewmodels.Return {
	if index < 0 || index >= len(returns) {
		return viewmodels.Return{}
	}
	return returns[index]
}

func returnCount(returns []viewmodels.Return, tab string) int {
	count := 0
	for _, item := range returns {
		if returnMatchesTab(item.Status, tab) {
			count++
		}
	}
	return count
}

func returnSubset(returns []viewmodels.Return, tab string) []viewmodels.Return {
	tab = strings.ToLower(strings.TrimSpace(tab))
	filtered := make([]viewmodels.Return, 0, len(returns))
	for _, item := range returns {
		if returnMatchesTab(item.Status, tab) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func returnMatchesTab(status, tab string) bool {
	status = strings.ToLower(strings.TrimSpace(status))
	switch strings.ToLower(strings.TrimSpace(tab)) {
	case "refunds":
		return status == "refunded" || strings.Contains(status, "refund")
	case "closed":
		return status == "declined" || status == "rejected" || status == "cancelled" || status == "closed"
	default:
		return !returnMatchesTab(status, "refunds") && !returnMatchesTab(status, "closed")
	}
}

func returnStatusLabel(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "pending":
		return "Pending review"
	case "approved":
		return "Approved"
	case "received":
		return "Item received"
	case "rejected":
		return "Declined"
	case "refunded":
		return "Refunded"
	default:
		return status
	}
}

func returnActionLabel(status string) string {
	if strings.Contains(strings.ToLower(strings.TrimSpace(status)), "refund") {
		return "View refund details"
	}
	return "View request"
}

func returnTabClass(filters url.Values, tab string) string {
	if strings.EqualFold(filters.Get("tab"), tab) || (tab == "" && filters.Get("tab") == "") {
		return "active"
	}
	return ""
}

func returnTabCurrent(filters url.Values, tab string) string {
	if returnTabClass(filters, tab) == "active" {
		return "page"
	}
	return ""
}

func productAt(products []viewmodels.Product, index int) viewmodels.Product {
	if index < 0 || index >= len(products) {
		return viewmodels.Product{}
	}
	return products[index]
}

func brandProducts(products []viewmodels.Product, brand string) []viewmodels.Product {
	result := make([]viewmodels.Product, 0)
	for _, product := range products {
		if strings.EqualFold(product.BrandName, brand) || strings.EqualFold(product.Seller, brand) {
			result = append(result, product)
		}
	}
	return result
}

func sellerProducts(products []viewmodels.Product, seller string) []viewmodels.Product {
	result := make([]viewmodels.Product, 0)
	for _, product := range products {
		if strings.EqualFold(product.Seller, seller) {
			result = append(result, product)
		}
	}
	return result
}

func paymentStateLabel(state string) string {
	switch state {
	case "success":
		return "Success"
	case "failed":
		return "Failed"
	default:
		return "Pending"
	}
}

func paymentStateTitle(state string) string {
	switch state {
	case "success":
		return "Payment successful"
	case "failed":
		return "Payment failed"
	default:
		return "Payment pending"
	}
}

func paymentStateDescription(state string) string {
	switch state {
	case "success":
		return "Your payment was captured and matched to the order."
	case "failed":
		return "The payment did not complete. Your order was not charged twice."
	default:
		return "The payment is waiting for confirmation from the payment network."
	}
}

func paymentStateHeading(state string) string {
	switch state {
	case "success":
		return "Payment captured"
	case "failed":
		return "Action needed"
	default:
		return "Reconciliation in progress"
	}
}

func paymentStateCopy(state string) string {
	switch state {
	case "success":
		return "The payment gateway confirmed the transaction and Weevcrafts has recorded it against your order."
	case "failed":
		return "You can retry with another method or contact support with the reference below. No duplicate order will be created."
	default:
		return "We are checking the gateway callback. If the status does not change, support can reconcile the reference manually."
	}
}

func paymentStateReference(state string) string {
	switch state {
	case "success":
		return "#PAY25603421"
	case "failed":
		return "#PAY25601432"
	default:
		return "#PAY25602912"
	}
}

func paymentStateNextStep(state string) string {
	switch state {
	case "success":
		return "No action required"
	case "failed":
		return "Retry or contact support"
	default:
		return "Wait for gateway confirmation"
	}
}

func paymentStateIcon(state string) string {
	switch state {
	case "success":
		return "check_circle"
	case "failed":
		return "error"
	default:
		return "hourglass_top"
	}
}

func paymentActivityClass(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "failed":
		return "danger"
	case "pending":
		return "review"
	default:
		return "success"
	}
}

func listingTitle(p viewmodels.CustomerPage) string {
	if p.Query != "" {
		return "Search results for “" + p.Query + "”"
	}
	if p.Category != "" {
		return strings.Title(strings.ReplaceAll(p.Category, "-", " "))
	}
	return "Explore handmade finds"
}

func listingDescription(p viewmodels.CustomerPage) string {
	if p.Query != "" {
		return "Thoughtfully made pieces that match your search, from verified Indian makers."
	}
	if p.Category != "" {
		return "Thoughtful pieces from independent Indian makers, made slowly and meant to last."
	}
	return "Timeless pieces and the stories of the people who make them."
}

func normalizedSort(sort string) string {
	if sort == "" {
		return "relevance"
	}
	return sort
}

func hasFilter(filters url.Values, key, value string) bool {
	for _, candidate := range filters[key] {
		if candidate == value || key == "category" && equivalentTextileCategory(candidate, value) {
			return true
		}
	}
	return false
}

func equivalentTextileCategory(left, right string) bool {
	return (left == "sarees" && right == "textiles") || (left == "textiles" && right == "sarees")
}

func filterCount(counts map[string]int, key, value string) string {
	if count := counts[key+":"+value]; count > 0 {
		return "(" + strconv.Itoa(count) + ")"
	}
	return ""
}

func listingURL(p viewmodels.CustomerPage, page int) string {
	values := url.Values{}
	for key, entries := range p.Filters {
		if key == "page" {
			continue
		}
		for _, entry := range entries {
			values.Add(key, entry)
		}
	}
	if page > 1 {
		values.Set("page", strconv.Itoa(page))
	}
	query := values.Encode()
	if query == "" {
		return "/products"
	}
	return "/products?" + query
}

func makerPath(name string) string {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "mithila arts":
		return "/makers/mithila-arts"
	case "weaver's touch":
		return "/makers/weavers-touch"
	case "clay & co.":
		return "/makers/clay-and-co"
	case "the rustic home":
		return "/makers/rustic-home"
	default:
		return "/makers/mithila-arts"
	}
}

func sellerPath(product viewmodels.Product) string {
	if product.SellerSlug != "" {
		return "/makers/" + url.PathEscape(product.SellerSlug)
	}
	return makerPath(product.Seller)
}

func productDescription(product viewmodels.Product) string {
	if description := strings.TrimSpace(product.Description); description != "" {
		return description
	}
	return "The seller has not provided a product description."
}

func productStockMessage(product viewmodels.Product) string {
	if product.InStock {
		return "In stock"
	}
	return "Currently unavailable"
}

func firstProductVariant(product viewmodels.Product) viewmodels.ProductVariant {
	if len(product.Variants) == 0 {
		return viewmodels.ProductVariant{}
	}
	return product.Variants[0]
}

func selectedProductVariant(variant viewmodels.ProductVariant, product viewmodels.Product) bool {
	return variant.ID != "" && variant.ID == strconv.FormatInt(product.VariantID, 10)
}
