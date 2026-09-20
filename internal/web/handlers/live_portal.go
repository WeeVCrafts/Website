package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/jackc/pgx/v5/pgxpool"

	applicationauth "github.com/wecratfs/commerce/internal/application/auth"
	"github.com/wecratfs/commerce/internal/web/viewmodels"
	adminpages "github.com/wecratfs/commerce/web/admin"
	"github.com/wecratfs/commerce/web/assets"
	sellerpages "github.com/wecratfs/commerce/web/selleradmin"
	supportpages "github.com/wecratfs/commerce/web/supportportal"
)

// LivePortalHandler renders the richer portal components against the
// authoritative PostgreSQL projections. Mutation routes are delegated to the
// existing application-backed handler so CSRF and service-level authorization
// remain in one place.
type LivePortalHandler struct {
	pool     *pgxpool.Pool
	sessions *applicationauth.SessionService
	fallback http.Handler
	assets   http.Handler
}

func NewLivePortalHandler(pool *pgxpool.Pool, sessions *applicationauth.SessionService, fallback http.Handler) *LivePortalHandler {
	return &LivePortalHandler{
		pool: pool, sessions: sessions, fallback: fallback,
		assets: http.StripPrefix("/assets/", http.FileServer(http.FS(assets.FS))),
	}
}

func (h *LivePortalHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/assets/") {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		h.assets.ServeHTTP(w, r)
		return
	}
	if r.Method != http.MethodGet {
		h.delegate(w, r)
		return
	}
	session, ok := h.session(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	switch {
	case r.URL.Path == "/admin" || strings.HasPrefix(r.URL.Path, "/admin/"):
		if !h.adminAllowed(r, session.UserID) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		h.renderAdmin(w, r, session.UserID)
	case r.URL.Path == "/seller-admin" || strings.HasPrefix(r.URL.Path, "/seller-admin/"):
		if !h.hasRole(r, session.UserID, "seller_owner", "seller_staff") {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		h.renderSeller(w, r, session.UserID)
	case r.URL.Path == "/support-portal" || strings.HasPrefix(r.URL.Path, "/support-portal/"):
		if !h.hasRole(r, session.UserID, "support_agent") {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		h.renderSupport(w, r, session.UserID)
	default:
		h.delegate(w, r)
	}
}

func (h *LivePortalHandler) adminAllowed(r *http.Request, userID int64) bool {
	if h.hasRole(r, userID, "marketplace_admin", "super_admin") {
		return true
	}
	path := strings.TrimPrefix(r.URL.Path, "/admin/")
	if h.hasRole(r, userID, "finance_operator") {
		return path == "finance" || path == "payments" || path == "failed-payments" || path == "refunds" || path == "finance-reconciliation" || path == "settlements" || path == "commission-rules"
	}
	if h.hasRole(r, userID, "operations_returns") {
		return path == "orders" || path == "returns" || path == "refunds" || path == "disputes"
	}
	return false
}

func (h *LivePortalHandler) session(r *http.Request) (struct {
	ID     string
	UserID int64
}, bool) {
	var result struct {
		ID     string
		UserID int64
	}
	if h.sessions == nil {
		return result, false
	}
	cookie, err := r.Cookie("wecratfs_session")
	if err != nil || cookie.Value == "" {
		return result, false
	}
	session, err := h.sessions.Get(r.Context(), cookie.Value)
	if err != nil || session.UserID <= 0 {
		return result, false
	}
	result.ID, result.UserID = session.ID, session.UserID
	return result, true
}

func (h *LivePortalHandler) hasRole(r *http.Request, userID int64, roles ...string) bool {
	if h.pool == nil || userID <= 0 || len(roles) == 0 {
		return false
	}
	var allowed bool
	if err := h.pool.QueryRow(r.Context(), `SELECT EXISTS (SELECT 1 FROM user_roles WHERE user_id=$1 AND role_slug = ANY($2::text[]))`, userID, roles).Scan(&allowed); err != nil {
		return false
	}
	return allowed
}

func (h *LivePortalHandler) renderAdmin(w http.ResponseWriter, r *http.Request, actorID int64) {
	page := h.adminPage(r, actorID)
	var component templ.Component
	if page.Workspace != "" && page.Workspace != page.Active {
		component = adminpages.Workspace(page)
	} else {
		switch page.Active {
		case "dashboard":
			component = adminpages.Dashboard(page)
		case "sellers":
			component = adminpages.Sellers(page)
		case "products":
			component = adminpages.Products(page)
		case "inventory":
			component = adminpages.Inventory(page)
		case "orders":
			component = adminpages.Orders(page)
		case "returns":
			component = adminpages.Returns(page)
		case "finance":
			component = adminpages.Finance(page)
		case "support":
			component = adminpages.Support(page)
		case "marketing":
			component = adminpages.Marketing(page)
		case "analytics":
			component = adminpages.Analytics(page)
		case "security":
			component = adminpages.Security(page)
		default:
			component = adminpages.Workspace(page)
		}
	}
	h.render(w, r, component)
}

func (h *LivePortalHandler) adminPage(r *http.Request, actorID int64) viewmodels.AdminPage {
	path := strings.TrimPrefix(r.URL.Path, "/admin")
	workspace := strings.TrimPrefix(path, "/")
	active := workspace
	if active == "" {
		active = "dashboard"
	}
	switch active {
	case "brands", "product-details", "category-management":
		active = "products"
	case "customers", "reviews":
		active = "support"
	case "payments", "failed-payments", "commission-rules", "settlements", "finance-reconciliation":
		active = "finance"
	case "disputes", "refunds":
		active = "returns"
	case "roles", "security-events":
		active = "security"
	case "audit-logs", "system-health", "search-indexing-health", "worker-queue-health", "settings":
		active = "analytics"
	case "seller-approvals", "seller-details", "seller-suspensions":
		active = "sellers"
	case "coupons":
		active = "marketing"
	}
	return h.adminData(r, actorID, active, workspace, r.URL.Query().Get("notice"), r.URL.Query().Get("q"))
}

func (h *LivePortalHandler) adminData(r *http.Request, actorID int64, active, workspace, notice, query string) viewmodels.AdminPage {
	page := viewmodels.AdminPage{Active: active, Workspace: workspace, Title: title(active), Subtitle: "Live marketplace operations backed by PostgreSQL.", Notice: notice, Query: query, Range: "Last 30 Days"}
	if workspace == "" {
		page.Title = "Marketplace dashboard"
	}
	page.Stats = h.adminStats(r)
	page.Bars = []viewmodels.AdminBar{{Label: "Week 1", Value: "Live order value", Height: "35", Color: "red"}, {Label: "Week 2", Value: "Live order value", Height: "50", Color: "green"}, {Label: "Week 3", Value: "Live order value", Height: "65", Color: "peach"}, {Label: "Week 4", Value: "Live order value", Height: "80", Color: "red"}}
	page.Sellers = h.adminSellers(r)
	page.Products = h.adminProducts(r)
	page.Orders = h.adminOrders(r)
	page.Returns = h.adminReturns(r)
	page.Transactions = h.adminTransactions(r)
	page.SupportCases = h.adminCases(r)
	page.Campaigns = h.adminCampaigns(r)
	page.Settlements = h.adminSettlements(r)
	page.AuditEntries = h.adminAudit(r)
	page.Configs = []viewmodels.AdminConfig{{Icon: "database", Name: "Authoritative store", Value: "PostgreSQL", Detail: "Business state source"}, {Icon: "search", Name: "Search index", Value: "Meilisearch", Detail: "Disposable read model"}, {Icon: "payments", Name: "Payment mode", Value: "Local demo", Detail: "Deterministic provider"}}
	_ = actorID
	return page
}

func (h *LivePortalHandler) adminStats(r *http.Request) []viewmodels.AdminStat {
	var sellers, products, orders, returns, tickets int64
	_ = h.pool.QueryRow(r.Context(), `SELECT COUNT(*) FROM sellers`).Scan(&sellers)
	_ = h.pool.QueryRow(r.Context(), `SELECT COUNT(*) FROM products`).Scan(&products)
	_ = h.pool.QueryRow(r.Context(), `SELECT COUNT(*) FROM orders`).Scan(&orders)
	_ = h.pool.QueryRow(r.Context(), `SELECT COUNT(*) FROM return_requests WHERE status='pending'`).Scan(&returns)
	_ = h.pool.QueryRow(r.Context(), `SELECT COUNT(*) FROM support_tickets WHERE status IN ('open','in_progress','waiting_customer')`).Scan(&tickets)
	return []viewmodels.AdminStat{{Icon: "groups", Value: strconv.FormatInt(sellers, 10), Label: "Sellers", Delta: "Live", Positive: true, Period: "database"}, {Icon: "inventory_2", Value: strconv.FormatInt(products, 10), Label: "Products", Delta: "Live", Positive: true, Period: "database"}, {Icon: "shopping_bag", Value: strconv.FormatInt(orders, 10), Label: "Orders", Delta: "Live", Positive: true, Period: "database"}, {Icon: "undo", Value: strconv.FormatInt(returns, 10), Label: "Pending returns", Delta: "Live", Period: "database"}, {Icon: "support_agent", Value: strconv.FormatInt(tickets, 10), Label: "Open support cases", Delta: "Live", Period: "database"}}
}

func (h *LivePortalHandler) adminSellers(r *http.Request) []viewmodels.AdminSeller {
	rows, err := h.pool.Query(r.Context(), `SELECT display_name, status, created_at FROM sellers ORDER BY id LIMIT 50`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var result []viewmodels.AdminSeller
	for rows.Next() {
		var name, status string
		var created time.Time
		if rows.Scan(&name, &status, &created) == nil {
			result = append(result, viewmodels.AdminSeller{Name: name, Status: status, Applied: created.Format("2 Jan 2006"), Image: "/assets/images/customer/maker-mithila.webp"})
		}
	}
	return result
}

func (h *LivePortalHandler) adminProducts(r *http.Request) []viewmodels.AdminProduct {
	rows, err := h.pool.Query(r.Context(), `SELECT p.name, COALESCE(pv.sku,''), COALESCE(s.display_name,''), c.name, p.price_cents, p.status, p.image_url FROM products p JOIN categories c ON c.slug=p.category_slug LEFT JOIN sellers s ON s.id=p.seller_id LEFT JOIN LATERAL (SELECT sku,price_cents FROM product_variants WHERE product_id=p.id ORDER BY id LIMIT 1) pv ON TRUE ORDER BY p.id LIMIT 50`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var result []viewmodels.AdminProduct
	for rows.Next() {
		var name, sku, seller, category, status, image string
		var price int64
		if rows.Scan(&name, &sku, &seller, &category, &price, &status, &image) == nil {
			result = append(result, viewmodels.AdminProduct{Name: name, SKU: sku, Seller: seller, Category: category, Price: money(price), Status: status, Image: image})
		}
	}
	return result
}

func (h *LivePortalHandler) adminOrders(r *http.Request) []viewmodels.AdminOrder {
	rows, err := h.pool.Query(r.Context(), `SELECT o.order_number, o.created_at, u.display_name, o.total_cents, o.status FROM orders o JOIN users u ON u.id=o.user_id ORDER BY o.created_at DESC LIMIT 50`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var result []viewmodels.AdminOrder
	for rows.Next() {
		var number, customer, status string
		var created time.Time
		var total int64
		if rows.Scan(&number, &created, &customer, &total, &status) == nil {
			result = append(result, viewmodels.AdminOrder{Number: number, Date: created.Format("2 Jan 2006, 15:04"), Customer: customer, Value: money(total), Fulfillment: status})
		}
	}
	return result
}

func (h *LivePortalHandler) adminReturns(r *http.Request) []viewmodels.AdminReturn {
	rows, err := h.pool.Query(r.Context(), `SELECT rr.id, o.order_number, u.display_name, rr.reason, rr.status, o.total_cents FROM return_requests rr JOIN orders o ON o.id=rr.order_id JOIN users u ON u.id=rr.user_id ORDER BY rr.created_at DESC LIMIT 50`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var result []viewmodels.AdminReturn
	for rows.Next() {
		var id int64
		var number, customer, reason, status string
		var amount int64
		if rows.Scan(&id, &number, &customer, &reason, &status, &amount) == nil {
			result = append(result, viewmodels.AdminReturn{Number: fmt.Sprintf("RTN-%d", id), Customer: customer, Product: number, Reason: reason, Status: status, Amount: money(amount), Image: "/assets/images/customer/terracotta-lamp.webp"})
		}
	}
	return result
}

func (h *LivePortalHandler) adminTransactions(r *http.Request) []viewmodels.AdminTransaction {
	rows, err := h.pool.Query(r.Context(), `SELECT pa.created_at, pa.status, o.order_number, pa.amount_cents FROM payment_attempts pa JOIN orders o ON o.id=pa.order_id ORDER BY pa.created_at DESC LIMIT 50`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var result []viewmodels.AdminTransaction
	for rows.Next() {
		var created time.Time
		var status, reference string
		var amount int64
		if rows.Scan(&created, &status, &reference, &amount) == nil {
			result = append(result, viewmodels.AdminTransaction{Date: created.Format("2 Jan 2006"), Type: "Payment", Reference: reference, Description: "Local provider payment", Amount: money(amount), Status: status})
		}
	}
	return result
}

func (h *LivePortalHandler) adminCases(r *http.Request) []viewmodels.AdminSupportCase {
	rows, err := h.pool.Query(r.Context(), `SELECT ticket_number, u.display_name, subject, priority, status, created_at FROM support_tickets t JOIN users u ON u.id=t.user_id ORDER BY t.created_at DESC LIMIT 50`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var result []viewmodels.AdminSupportCase
	for rows.Next() {
		var number, customer, subject, priority, status string
		var created time.Time
		if rows.Scan(&number, &customer, &subject, &priority, &status, &created) == nil {
			result = append(result, viewmodels.AdminSupportCase{Case: number, Customer: customer, Subject: subject, Priority: priority, Status: status, Date: created.Format("2 Jan 2006")})
		}
	}
	return result
}

func (h *LivePortalHandler) adminCampaigns(r *http.Request) []viewmodels.AdminCampaign {
	rows, err := h.pool.Query(r.Context(), `SELECT name, status, COALESCE(starts_at,NOW()), COALESCE(ends_at,NOW()) FROM promotions ORDER BY id DESC LIMIT 20`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var result []viewmodels.AdminCampaign
	for rows.Next() {
		var name, status string
		var start, end time.Time
		if rows.Scan(&name, &status, &start, &end) == nil {
			result = append(result, viewmodels.AdminCampaign{Name: name, Status: status, Start: start.Format("2 Jan 2006"), End: end.Format("2 Jan 2006"), Type: "Promotion", Channel: "Marketplace"})
		}
	}
	return result
}

func (h *LivePortalHandler) adminSettlements(r *http.Request) []viewmodels.AdminSettlement {
	rows, err := h.pool.Query(r.Context(), `SELECT s.display_name, ss.net_cents, ss.status, ss.period_end FROM seller_settlements ss JOIN sellers s ON s.id=ss.seller_id ORDER BY ss.id DESC LIMIT 20`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var result []viewmodels.AdminSettlement
	for rows.Next() {
		var seller, status string
		var amount int64
		var date time.Time
		if rows.Scan(&seller, &amount, &status, &date) == nil {
			result = append(result, viewmodels.AdminSettlement{Seller: seller, Amount: money(amount), Status: status, Date: date.Format("2 Jan 2006")})
		}
	}
	return result
}

func (h *LivePortalHandler) adminAudit(r *http.Request) []viewmodels.AdminAuditEntry {
	rows, err := h.pool.Query(r.Context(), `SELECT created_at, COALESCE(actor_id::text,''), action, resource_type, resource_id FROM audit_logs ORDER BY id DESC LIMIT 50`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var result []viewmodels.AdminAuditEntry
	for rows.Next() {
		var created time.Time
		var actor, action, kind, resource string
		if rows.Scan(&created, &actor, &action, &kind, &resource) == nil {
			result = append(result, viewmodels.AdminAuditEntry{Date: created.Format("2 Jan 2006, 15:04"), User: actor, Action: action, Entity: kind, Details: resource})
		}
	}
	return result
}

func (h *LivePortalHandler) renderSeller(w http.ResponseWriter, r *http.Request, userID int64) {
	page := h.sellerPage(r, userID)
	var component templ.Component
	switch page.Active {
	case "dashboard":
		component = sellerpages.Dashboard(page)
	case "onboarding":
		component = sellerpages.Onboarding(page)
	case "products":
		component = sellerpages.Products(page)
	case "inventory":
		component = sellerpages.Inventory(page)
	case "orders":
		component = sellerpages.Orders(page)
	case "returns":
		component = sellerpages.Returns(page)
	case "earnings":
		component = sellerpages.Earnings(page)
	case "analytics":
		component = sellerpages.Analytics(page)
	case "settings":
		component = sellerpages.Settings(page)
	case "team":
		component = sellerpages.Team(page)
	default:
		component = sellerpages.Workspace(page)
	}
	h.render(w, r, component)
}

func (h *LivePortalHandler) sellerPage(r *http.Request, userID int64) viewmodels.SellerAdminPage {
	path := strings.TrimPrefix(r.URL.Path, "/seller-admin")
	active := strings.Trim(path, "/")
	if active == "" {
		active = "dashboard"
	}
	var sellerID int64
	var name string
	if err := h.pool.QueryRow(r.Context(), `SELECT s.id,s.display_name FROM sellers s WHERE s.owner_user_id=$1 OR EXISTS (SELECT 1 FROM seller_users su WHERE su.seller_id=s.id AND su.user_id=$1 AND su.status='active') ORDER BY CASE WHEN s.owner_user_id=$1 THEN 0 ELSE 1 END,s.id LIMIT 1`, userID).Scan(&sellerID, &name); err != nil {
		return viewmodels.SellerAdminPage{Active: active, Title: "Seller workspace", Subtitle: "No seller profile is assigned to this account."}
	}
	page := viewmodels.SellerAdminPage{Active: active, Workspace: active, Title: title(active), Subtitle: name + " · live seller workspace", Query: r.URL.Query().Get("q"), Notice: r.URL.Query().Get("notice")}
	page.Stats = h.sellerStats(r, sellerID)
	page.Products = h.sellerProducts(r, sellerID)
	page.Inventory = h.sellerInventory(r, sellerID)
	page.Orders = h.sellerOrders(r, sellerID)
	if h.isSellerOwner(r, userID, sellerID) {
		page.Ledger = h.sellerLedger(r, sellerID)
		page.Settlements = h.sellerSettlements(r, sellerID)
		page.Team = h.sellerTeam(r, sellerID)
	}
	return page
}

func (h *LivePortalHandler) isSellerOwner(r *http.Request, userID, sellerID int64) bool {
	var owner bool
	_ = h.pool.QueryRow(r.Context(), `SELECT EXISTS (SELECT 1 FROM sellers WHERE id=$1 AND owner_user_id=$2)`, sellerID, userID).Scan(&owner)
	return owner
}

func (h *LivePortalHandler) sellerStats(r *http.Request, sellerID int64) []viewmodels.SellerAdminStat {
	var products, orders, stock int64
	_ = h.pool.QueryRow(r.Context(), `SELECT COUNT(*) FROM products WHERE seller_id=$1`, sellerID).Scan(&products)
	_ = h.pool.QueryRow(r.Context(), `SELECT COUNT(DISTINCT sf.order_id) FROM seller_fulfillments sf WHERE sf.seller_id=$1`, sellerID).Scan(&orders)
	_ = h.pool.QueryRow(r.Context(), `SELECT COALESCE(SUM(i.available_quantity),0) FROM inventory_stock i JOIN product_variants v ON v.id=i.variant_id JOIN products p ON p.id=v.product_id WHERE p.seller_id=$1`, sellerID).Scan(&stock)
	return []viewmodels.SellerAdminStat{{Icon: "category", Value: strconv.FormatInt(products, 10), Label: "Products", Delta: "Live", Positive: true, Period: "database", Tone: "green"}, {Icon: "inventory_2", Value: strconv.FormatInt(orders, 10), Label: "Orders", Delta: "Live", Positive: true, Period: "database", Tone: "blue"}, {Icon: "warehouse", Value: strconv.FormatInt(stock, 10), Label: "Available stock", Delta: "Live", Positive: true, Period: "database", Tone: "orange"}}
}

func (h *LivePortalHandler) sellerProducts(r *http.Request, sellerID int64) []viewmodels.SellerAdminProduct {
	rows, err := h.pool.Query(r.Context(), `SELECT p.name, COALESCE(v.sku,''), c.name, p.price_cents, p.status, COALESCE(i.available_quantity,0) FROM products p JOIN categories c ON c.slug=p.category_slug LEFT JOIN LATERAL (SELECT id,sku,price_cents FROM product_variants WHERE product_id=p.id ORDER BY id LIMIT 1) v ON TRUE LEFT JOIN inventory_stock i ON i.variant_id=v.id WHERE p.seller_id=$1 ORDER BY p.id LIMIT 50`, sellerID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var result []viewmodels.SellerAdminProduct
	for rows.Next() {
		var name, sku, category, status string
		var price, stock int64
		if rows.Scan(&name, &sku, &category, &price, &status, &stock) == nil {
			result = append(result, viewmodels.SellerAdminProduct{Name: name, SKU: sku, Category: category, Price: money(price), Stock: strconv.FormatInt(stock, 10), Status: status, Image: "/assets/images/customer/madhubani-tree.webp"})
		}
	}
	return result
}

func (h *LivePortalHandler) sellerInventory(r *http.Request, sellerID int64) []viewmodels.SellerAdminInventory {
	rows, err := h.pool.Query(r.Context(), `SELECT p.name,v.sku,COALESCE(i.available_quantity,0),COALESCE(i.reserved_quantity,0) FROM products p JOIN product_variants v ON v.product_id=p.id LEFT JOIN inventory_stock i ON i.variant_id=v.id WHERE p.seller_id=$1 ORDER BY p.id LIMIT 50`, sellerID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var result []viewmodels.SellerAdminInventory
	for rows.Next() {
		var name, sku string
		var available, reserved int64
		if rows.Scan(&name, &sku, &available, &reserved) == nil {
			result = append(result, viewmodels.SellerAdminInventory{Name: name, SKU: sku, Variant: "Default variant", Available: strconv.FormatInt(available, 10), Reserved: strconv.FormatInt(reserved, 10), Status: "In Stock", Image: "/assets/images/customer/madhubani-tree.webp"})
		}
	}
	return result
}

func (h *LivePortalHandler) sellerOrders(r *http.Request, sellerID int64) []viewmodels.SellerAdminOrder {
	rows, err := h.pool.Query(r.Context(), `
		SELECT o.order_number, o.created_at, u.display_name, COALESCE(SUM(oi.line_total_cents),0),
		       COALESCE(sf.status,'pending'), COALESCE(pa.status,'created'), COALESCE(SUM(oi.quantity),0)
		FROM orders o
		JOIN users u ON u.id=o.user_id
		JOIN order_items oi ON oi.order_id=o.id
		JOIN product_variants pv ON pv.id=oi.variant_id
		JOIN products p ON p.id=pv.product_id AND p.seller_id=$1
		LEFT JOIN seller_fulfillments sf ON sf.order_id=o.id AND sf.seller_id=$1
		LEFT JOIN LATERAL (SELECT status FROM payment_attempts WHERE order_id=o.id ORDER BY id DESC LIMIT 1) pa ON TRUE
		WHERE o.status IN ('paid','processing','shipped','delivered')
		GROUP BY o.id, u.display_name, sf.status, pa.status
		ORDER BY o.created_at DESC, o.id DESC
		LIMIT 50`, sellerID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var result []viewmodels.SellerAdminOrder
	for rows.Next() {
		var number, customer, status, payment string
		var created time.Time
		var total, items int64
		if rows.Scan(&number, &created, &customer, &total, &status, &payment, &items) == nil {
			result = append(result, viewmodels.SellerAdminOrder{Number: number, Date: created.Format("2 Jan 2006"), Customer: customer, Items: strconv.FormatInt(items, 10), Value: money(total), Status: status, Payment: displayPaymentStatus(payment, ""), Image: "/assets/images/customer/madhubani-tree.webp"})
		}
	}
	return result
}

func (h *LivePortalHandler) sellerLedger(r *http.Request, sellerID int64) []viewmodels.SellerAdminLedger {
	rows, err := h.pool.Query(r.Context(), `SELECT id,created_at,gross_cents,commission_cents,net_cents,status FROM seller_ledger WHERE seller_id=$1 ORDER BY id DESC LIMIT 50`, sellerID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var result []viewmodels.SellerAdminLedger
	for rows.Next() {
		var id int64
		var created time.Time
		var gross, commission, net int64
		var status string
		if rows.Scan(&id, &created, &gross, &commission, &net, &status) == nil {
			result = append(result, viewmodels.SellerAdminLedger{Number: fmt.Sprintf("LEDGER-%d", id), Date: created.Format("2 Jan 2006"), Sale: money(gross), Commission: money(commission), Net: money(net), Status: status})
		}
	}
	return result
}

func (h *LivePortalHandler) sellerSettlements(r *http.Request, sellerID int64) []viewmodels.SellerAdminSettlement {
	rows, err := h.pool.Query(r.Context(), `SELECT reference,net_cents,status,period_end FROM seller_settlements WHERE seller_id=$1 ORDER BY id DESC LIMIT 20`, sellerID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var result []viewmodels.SellerAdminSettlement
	for rows.Next() {
		var ref, status string
		var amount int64
		var date time.Time
		if rows.Scan(&ref, &amount, &status, &date) == nil {
			result = append(result, viewmodels.SellerAdminSettlement{Number: ref, Amount: money(amount), Status: status, Date: date.Format("2 Jan 2006")})
		}
	}
	return result
}

func (h *LivePortalHandler) sellerTeam(r *http.Request, sellerID int64) []viewmodels.SellerTeamMember {
	rows, err := h.pool.Query(r.Context(), `SELECT u.display_name,u.email,su.status,su.permissions FROM seller_users su JOIN users u ON u.id=su.user_id WHERE su.seller_id=$1 ORDER BY u.id`, sellerID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var result []viewmodels.SellerTeamMember
	for rows.Next() {
		var name, email, status string
		var permissions []string
		if rows.Scan(&name, &email, &status, &permissions) == nil {
			result = append(result, viewmodels.SellerTeamMember{Name: name, Email: email, Role: "Seller staff", Status: status, Permissions: strings.Join(permissions, ", "), LastActive: "Live account"})
		}
	}
	return result
}

func (h *LivePortalHandler) renderSupport(w http.ResponseWriter, r *http.Request, actorID int64) {
	page := h.supportPage(r, actorID)
	var component templ.Component
	switch {
	case strings.HasSuffix(r.URL.Path, "/customers"):
		component = supportpages.Customers(page)
	case strings.HasSuffix(r.URL.Path, "/orders"):
		component = supportpages.Orders(page)
	case strings.HasSuffix(r.URL.Path, "/cases"):
		component = supportpages.Cases(page)
	default:
		component = supportpages.Dashboard(page)
	}
	h.render(w, r, component)
}

func (h *LivePortalHandler) supportPage(r *http.Request, actorID int64) viewmodels.SupportPortalPage {
	_ = actorID
	active := "dashboard"
	trimmed := strings.Trim(strings.TrimPrefix(r.URL.Path, "/support-portal"), "/")
	if trimmed != "" {
		active = trimmed
	}
	page := viewmodels.SupportPortalPage{Active: active, Title: title(active), Subtitle: "Masked customer support workspace backed by live tickets.", Query: r.URL.Query().Get("q"), Notice: r.URL.Query().Get("notice"), Tab: r.URL.Query().Get("tab")}
	page.Cases = h.supportCases(r)
	page.Stats = []viewmodels.SupportPortalStat{{Icon: "support_agent", Value: strconv.Itoa(len(page.Cases)), Label: "Visible cases", Delta: "Live", Positive: true, Period: "database", Tone: "green"}}
	return page
}

func (h *LivePortalHandler) supportCases(r *http.Request) []viewmodels.SupportPortalCase {
	rows, err := h.pool.Query(r.Context(), `SELECT t.ticket_number,t.subject,u.display_name,t.priority,t.created_at,t.status,COALESCE(o.order_number,'') FROM support_tickets t JOIN users u ON u.id=t.user_id LEFT JOIN orders o ON o.id=t.order_id WHERE t.status <> 'closed' ORDER BY t.created_at DESC LIMIT 100`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var result []viewmodels.SupportPortalCase
	for rows.Next() {
		var number, subject, customer, priority, status, orderNumber string
		var created time.Time
		if rows.Scan(&number, &subject, &customer, &priority, &created, &status, &orderNumber) == nil {
			result = append(result, viewmodels.SupportPortalCase{ID: number, Subject: subject, Customer: maskName(customer), CustomerInitials: initials(customer), Category: "Order support", Priority: priority, Created: created.Format("2 Jan 2006 15:04"), Status: status, Order: orderNumber, Assignee: "Support team", Channel: "Portal"})
		}
	}
	return result
}

func (h *LivePortalHandler) render(w http.ResponseWriter, r *http.Request, component templ.Component) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := component.Render(r.Context(), w); err != nil {
		http.Error(w, "portal rendering failed", http.StatusInternalServerError)
	}
}

func (h *LivePortalHandler) delegate(w http.ResponseWriter, r *http.Request) {
	if h.fallback != nil {
		h.fallback.ServeHTTP(w, r)
		return
	}
	http.NotFound(w, r)
}

func title(value string) string {
	if value == "" {
		return "Workspace"
	}
	return strings.ToUpper(value[:1]) + value[1:]
}
func money(cents int64) string { return fmt.Sprintf("INR %.2f", float64(cents)/100) }
func maskName(value string) string {
	if value == "" {
		return "Hidden customer"
	}
	parts := strings.Fields(value)
	if len(parts) == 1 {
		return parts[0][:1] + "***"
	}
	return parts[0] + " " + parts[len(parts)-1][:1] + "."
}
func initials(value string) string {
	parts := strings.Fields(value)
	if len(parts) == 0 {
		return "--"
	}
	result := string(parts[0][0])
	if len(parts) > 1 {
		result += string(parts[len(parts)-1][0])
	}
	return result
}
