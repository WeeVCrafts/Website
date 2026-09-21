package handlers

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/jackc/pgx/v5/pgxpool"

	applicationauth "github.com/wecratfs/commerce/internal/application/auth"
	domainidentity "github.com/wecratfs/commerce/internal/domain/identity"
	"github.com/wecratfs/commerce/internal/web/viewmodels"
	adminpages "github.com/wecratfs/commerce/web/admin"
	"github.com/wecratfs/commerce/web/assets"
	sellerpages "github.com/wecratfs/commerce/web/selleradmin"
	supportpages "github.com/wecratfs/commerce/web/supportportal"
	workspacepages "github.com/wecratfs/commerce/web/workspaces"
)

// LivePortalHandler renders the richer portal components against the
// authoritative PostgreSQL projections. Mutation routes are delegated to the
// existing application-backed handler so CSRF and service-level authorization
// remain in one place.
type LivePortalHandler struct {
	pool      *pgxpool.Pool
	sessions  *applicationauth.SessionService
	roleAdmin *applicationauth.RoleAdminService
	fallback  http.Handler
	assets    http.Handler
}

func (h *LivePortalHandler) SetRoleAdminService(service *applicationauth.RoleAdminService) {
	h.roleAdmin = service
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
		if legacyPath := canonicalAdminMutationPath(r.URL.Path); legacyPath != "" {
			_, root, _ := canonicalAdminPortal(r.URL.Path)
			h.delegateWithPortalRedirect(w, requestWithPath(r, legacyPath), root)
			return
		}
		if legacyPath := canonicalSellerMutationPath(r.URL.Path); legacyPath != "" {
			session, ok := h.session(r)
			if !ok {
				redirectToLogin(w, r)
				return
			}
			roles := h.userRoles(r, session.UserID)
			isOwner := hasIdentityRole(roles, domainidentity.RoleSellerOwner)
			permissions, assigned := h.sellerPermissions(r, session.UserID)
			if !assigned || !sellerMutationAllowed(r.URL.Path, isOwner, permissions) {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			h.delegateWithPortalRedirect(w, requestWithPath(r, legacyPath), "/seller-admin")
			return
		}
		if r.URL.Path == "/support-portal/cases/status" {
			session, ok := h.session(r)
			if !ok {
				redirectToLogin(w, r)
				return
			}
			if !h.hasRole(r, session.UserID, "support_agent") {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			h.delegateWithPortalRedirect(w, requestWithPath(r, "/support/status"), "/support-portal")
			return
		}
		if r.URL.Path == "/support-portal/cases/reply" {
			session, ok := h.session(r)
			if !ok {
				redirectToLogin(w, r)
				return
			}
			if !h.hasRole(r, session.UserID, "support_agent") {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			h.delegateWithPortalRedirect(w, requestWithPath(r, "/support/reply"), "/support-portal")
			return
		}
		h.delegate(w, r)
		return
	}
	if r.URL.Path == "/workspaces" {
		session, ok := h.session(r)
		if !ok {
			redirectToLogin(w, r)
			return
		}
		h.renderWorkspaces(w, r, session)
		return
	}
	session, ok := h.session(r)
	if !ok {
		redirectToLogin(w, r)
		return
	}
	if pathUnder(r.URL.Path, "/seller") {
		destination := legacySellerPortalDestination(r.URL.Path)
		roles := h.userRoles(r, session.UserID)
		isOwner := hasIdentityRole(roles, domainidentity.RoleSellerOwner)
		isStaff := hasIdentityRole(roles, domainidentity.RoleSellerStaff)
		permissions, assigned := h.sellerPermissions(r, session.UserID)
		if destination == "" || (!isOwner && !isStaff) || !assigned || !sellerPortalAllowed(destination, isOwner, permissions) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		redirectPortalDestination(w, r, destination)
		return
	}
	if pathUnder(r.URL.Path, "/support") {
		destination := legacySupportPortalDestination(r.URL.Path)
		if destination == "" || !h.hasRole(r, session.UserID, "support_agent") {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		redirectPortalDestination(w, r, destination)
		return
	}
	if r.URL.Path == "/admin" || strings.HasPrefix(r.URL.Path, "/admin/") || r.URL.Path == "/finance" || r.URL.Path == "/finance/" {
		roles := h.userRoles(r, session.UserID)
		destination := legacyAdminDestination(r.URL.Path, roles)
		if destination == "" {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if r.URL.RawQuery != "" {
			destination += "?" + r.URL.RawQuery
		}
		http.Redirect(w, r, destination, http.StatusSeeOther)
		return
	}
	switch {
	case isCanonicalAdminPortal(r.URL.Path):
		role, root, _ := canonicalAdminPortal(r.URL.Path)
		if !roleAllowsPortalPath(role, r.URL.Path) || !h.hasRole(r, session.UserID, roleString(role)) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if role == domainidentity.RoleSuperAdmin && isSuperAdminProductEditorPath(r.URL.Path) {
			h.renderSuperAdminProductEditor(w, r, session.UserID, h.userDisplayName(r, session.UserID), session.CSRFToken)
			return
		}
		legacyPath := adminLegacyPagePath(r.URL.Path, root)
		pageRequest := requestWithPath(r, legacyPath)
		name := h.userDisplayName(r, session.UserID)
		h.renderAdmin(w, pageRequest, session.UserID, role, root, name, session.CSRFToken)
	case r.URL.Path == "/seller-admin" || strings.HasPrefix(r.URL.Path, "/seller-admin/"):
		roles := h.userRoles(r, session.UserID)
		isOwner := hasIdentityRole(roles, domainidentity.RoleSellerOwner)
		if !isOwner && !hasIdentityRole(roles, domainidentity.RoleSellerStaff) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		permissions, assigned := h.sellerPermissions(r, session.UserID)
		if !assigned || !sellerPortalAllowed(r.URL.Path, isOwner, permissions) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		name := h.userDisplayName(r, session.UserID)
		if r.URL.Path == "/seller-admin/products/new" || isSellerProductFormPath(r.URL.Path) {
			h.renderSellerProductForm(w, r, session.UserID, name, isOwner, permissions, session.CSRFToken)
			return
		}
		h.renderSeller(w, r, session.UserID, name, isOwner, permissions, session.CSRFToken)
	case r.URL.Path == "/support-portal" || strings.HasPrefix(r.URL.Path, "/support-portal/"):
		if !h.hasRole(r, session.UserID, "support_agent") {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		h.renderSupport(w, r, session.UserID, session.CSRFToken)
	default:
		h.delegate(w, r)
	}
}

func (h *LivePortalHandler) session(r *http.Request) (struct {
	ID        string
	UserID    int64
	CSRFToken string
}, bool) {
	var result struct {
		ID        string
		UserID    int64
		CSRFToken string
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
	result.ID, result.UserID, result.CSRFToken = session.ID, session.UserID, session.CSRFToken
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

func (h *LivePortalHandler) userRoles(r *http.Request, userID int64) []domainidentity.Role {
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

func (h *LivePortalHandler) userDisplayName(r *http.Request, userID int64) string {
	if h.pool == nil || userID <= 0 {
		return "WeeVCrafts account"
	}
	var name string
	if err := h.pool.QueryRow(r.Context(), `SELECT display_name FROM users WHERE id=$1`, userID).Scan(&name); err != nil || strings.TrimSpace(name) == "" {
		return "WeeVCrafts account"
	}
	return name
}

func (h *LivePortalHandler) userEmail(r *http.Request, userID int64) string {
	if h.pool == nil || userID <= 0 {
		return ""
	}
	var email string
	_ = h.pool.QueryRow(r.Context(), `SELECT email FROM users WHERE id=$1`, userID).Scan(&email)
	return email
}

func (h *LivePortalHandler) sellerPermissions(r *http.Request, userID int64) ([]string, bool) {
	if h.pool == nil || userID <= 0 {
		return nil, false
	}
	var permissions []string
	var assigned bool
	err := h.pool.QueryRow(r.Context(), `
		SELECT COALESCE(su.permissions, ARRAY[]::text[]), (s.owner_user_id=$1 OR su.user_id=$1)
		FROM sellers s
		LEFT JOIN seller_users su ON su.seller_id=s.id AND su.user_id=$1 AND su.status='active'
		WHERE s.owner_user_id=$1 OR su.user_id=$1
		ORDER BY CASE WHEN s.owner_user_id=$1 THEN 0 ELSE 1 END,s.id
		LIMIT 1`, userID).Scan(&permissions, &assigned)
	if err != nil {
		return nil, false
	}
	return permissions, assigned
}

func (h *LivePortalHandler) renderWorkspaces(w http.ResponseWriter, r *http.Request, session struct {
	ID        string
	UserID    int64
	CSRFToken string
}) {
	roles := h.userRoles(r, session.UserID)
	choices := applicationauth.WorkspacesForRoles(roles)
	page := viewmodels.WorkspacePage{DisplayName: h.userDisplayName(r, session.UserID), CSRFToken: session.CSRFToken}
	for _, choice := range choices {
		page.Workspaces = append(page.Workspaces, viewmodels.WorkspaceChoice{Role: string(choice.Role), Label: choice.Label, Path: choice.Path})
	}
	h.render(w, r, workspacepages.Document(page))
}

func (h *LivePortalHandler) renderAdmin(w http.ResponseWriter, r *http.Request, actorID int64, role domainidentity.Role, root, displayName, csrfToken string) {
	page := h.adminPage(r, actorID, role)
	page.PortalRoot = root
	page.PortalLabel = portalLabel(role)
	page.RoleLabel = roleLabel(role)
	page.DisplayName = displayName
	page.CSRFToken = csrfToken
	page.CanFinance = role == domainidentity.RoleSuperAdmin || role == domainidentity.RoleFinanceOperator
	page.CanSecurity = role == domainidentity.RoleSuperAdmin
	page.CanManageCatalog = role == domainidentity.RoleSuperAdmin
	page.IsFinance = role == domainidentity.RoleFinanceOperator
	page.IsOperations = role == domainidentity.RoleOperations
	page.NavActive = page.Active
	if page.IsFinance && page.Workspace != "" {
		page.NavActive = page.Workspace
	}
	page.Stats = h.portalStats(r, role)
	if role == domainidentity.RoleFinanceOperator {
		switch page.Workspace {
		case "failed-payments":
			page.Transactions = keepAdminTransactions(page.Transactions, "failed")
		case "refunds":
			page.Transactions = keepAdminTransactions(page.Transactions, "refunded")
		case "settlements":
			page.Transactions = nil
		}
	}
	var component templ.Component
	switch page.Active {
	case "dashboard":
		component = adminpages.PortalDashboard(page)
	case "sellers":
		component = adminpages.PortalSellers(page)
	case "products":
		component = adminpages.PortalProducts(page)
	case "inventory":
		page.Inventory = h.adminInventory(r)
		component = adminpages.PortalInventory(page)
	case "orders":
		component = adminpages.PortalOrders(page)
	case "returns":
		component = adminpages.PortalReturns(page)
	case "finance":
		component = adminpages.PortalFinance(page)
	case "support":
		component = adminpages.PortalSupport(page)
	case "marketing":
		component = adminpages.PortalMarketing(page)
	case "analytics":
		component = adminpages.PortalAnalytics(page)
	case "security":
		component = adminpages.PortalSecurity(page)
	default:
		component = adminpages.Workspace(page)
	}
	h.renderAdminComponent(w, r, component, page)
}

func isSuperAdminProductEditorPath(path string) bool {
	if path == "/super-admin/products/new" {
		return true
	}
	if !strings.HasPrefix(path, "/super-admin/products/") || !strings.HasSuffix(path, "/edit") {
		return false
	}
	id := strings.TrimSuffix(strings.TrimPrefix(path, "/super-admin/products/"), "/edit")
	parsed, err := strconv.ParseInt(id, 10, 64)
	return err == nil && parsed > 0 && !strings.Contains(id, "/")
}

func (h *LivePortalHandler) renderSuperAdminProductEditor(w http.ResponseWriter, r *http.Request, actorID int64, displayName, csrfToken string) {
	pageRequest := requestWithPath(r, "/admin/products")
	page := h.adminPage(pageRequest, actorID, domainidentity.RoleSuperAdmin)
	page.PortalRoot = "/super-admin"
	page.PortalLabel = portalLabel(domainidentity.RoleSuperAdmin)
	page.RoleLabel = roleLabel(domainidentity.RoleSuperAdmin)
	page.DisplayName = displayName
	page.CSRFToken = csrfToken
	page.CanFinance, page.CanSecurity, page.CanManageCatalog = true, true, true
	page.Stats = h.portalStats(r, domainidentity.RoleSuperAdmin)
	page.Sellers = h.adminSellers(r)

	form := viewmodels.AdminProductEditor{Action: r.URL.Path, InitialStock: "0", DeliveryLabel: "Delivery to serviceable PIN codes in India"}
	if r.URL.Path == "/super-admin/products/new" {
		page.Title = "Add product"
		page.Subtitle = "Create a product draft for an active Indian seller, then submit it for marketplace review."
	} else {
		idText := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/super-admin/products/"), "/edit")
		productID, err := strconv.ParseInt(idText, 10, 64)
		if err != nil || productID <= 0 {
			http.NotFound(w, r)
			return
		}
		var id, sellerID, price, compareAt, stock int64
		var status string
		var hasCompareAt bool
		err = h.pool.QueryRow(r.Context(), `
			SELECT p.id,p.seller_id,COALESCE(s.display_name,''),p.name,p.slug,p.brand_slug,b.name,p.category_slug,c.name,p.description,
			       COALESCE(v.sku,''),COALESCE(v.price_cents,p.price_cents),COALESCE(p.compare_at_cents,0),p.compare_at_cents IS NOT NULL,
			       COALESCE(i.available_quantity,0),p.delivery_label,p.status,p.image_url
			FROM products p LEFT JOIN sellers s ON s.id=p.seller_id
			JOIN brands b ON b.slug=p.brand_slug JOIN categories c ON c.slug=p.category_slug
			LEFT JOIN LATERAL (SELECT id,sku,price_cents FROM product_variants WHERE product_id=p.id AND status='active' ORDER BY id LIMIT 1) v ON TRUE
			LEFT JOIN inventory_stock i ON i.variant_id=v.id WHERE p.id=$1`, productID).Scan(&id, &sellerID, &form.SellerName, &form.Name, &form.Slug, &form.BrandSlug, &form.BrandName, &form.CategorySlug, &form.CategoryName, &form.Description, &form.SKU, &price, &compareAt, &hasCompareAt, &stock, &form.DeliveryLabel, &status, &form.Image)
		if err != nil || status == "archived" {
			http.NotFound(w, r)
			return
		}
		form.ID = strconv.FormatInt(id, 10)
		form.SellerID = strconv.FormatInt(sellerID, 10)
		form.PriceCents = strconv.FormatInt(price, 10)
		form.InitialStock = strconv.FormatInt(stock, 10)
		if hasCompareAt {
			form.CompareAtCents = strconv.FormatInt(compareAt, 10)
		}
		form.Variants, err = h.productVariantViews(r, id)
		if err != nil {
			http.Error(w, "Could not load product variants", http.StatusServiceUnavailable)
			return
		}
		form.Media, err = productMediaViews(r.Context(), h.pool, id)
		if err != nil {
			http.Error(w, "Could not load product media", http.StatusServiceUnavailable)
			return
		}
		page.Title = "Edit product"
		page.Subtitle = "Update all listing details. Stock remains in the permission-scoped Inventory workflow."
	}
	h.render(w, r, adminpages.ProductEditor(page, form))
}

func (h *LivePortalHandler) adminPage(r *http.Request, actorID int64, role domainidentity.Role) viewmodels.AdminPage {
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
	if role == domainidentity.RoleFinanceOperator && workspace == "refunds" {
		active = "finance"
	}
	return h.adminData(r, actorID, active, workspace, r.URL.Query().Get("notice"), r.URL.Query().Get("q"))
}

func (h *LivePortalHandler) adminData(r *http.Request, actorID int64, active, workspace, notice, query string) viewmodels.AdminPage {
	page := viewmodels.AdminPage{ActorID: actorID, Active: active, Workspace: workspace, Title: title(active), Subtitle: "Live marketplace operations backed by PostgreSQL.", Notice: notice, Query: query, Range: "Last 30 Days"}
	if workspace == "" {
		page.Title = "Marketplace dashboard"
	}
	page.Stats = h.adminStats(r)
	page.Bars = []viewmodels.AdminBar{{Label: "Week 1", Value: "Live order value", Height: "35", Color: "red"}, {Label: "Week 2", Value: "Live order value", Height: "50", Color: "green"}, {Label: "Week 3", Value: "Live order value", Height: "65", Color: "peach"}, {Label: "Week 4", Value: "Live order value", Height: "80", Color: "red"}}
	page.Sellers = h.adminSellers(r)
	page.Products = h.adminProducts(r, portalQuery(query))
	page.Orders = h.adminOrders(r)
	page.Returns = h.adminReturns(r)
	page.Transactions = h.adminTransactions(r)
	page.SupportCases = h.adminCases(r)
	page.Campaigns = h.adminCampaigns(r)
	page.Settlements = h.adminSettlements(r)
	page.AuditEntries = h.adminAudit(r)
	page.Configs = []viewmodels.AdminConfig{{Icon: "database", Name: "Authoritative store", Value: "PostgreSQL", Detail: "Business state source"}, {Icon: "search", Name: "Search index", Value: "Meilisearch", Detail: "Disposable read model"}, {Icon: "payments", Name: "Payment mode", Value: "Local demo", Detail: "Deterministic provider"}}
	filterAdminPage(&page, query)
	if active == "security" && h.roleAdmin != nil {
		assignments, err := h.roleAdmin.List(r.Context(), actorID, query)
		if err != nil {
			page.Notice = "Role assignments could not be loaded."
		} else {
			page.RoleAssignments = assignments
		}
	}
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

func (h *LivePortalHandler) portalStats(r *http.Request, role domainidentity.Role) []viewmodels.AdminStat {
	count := func(query string, label, icon string) viewmodels.AdminStat {
		var value int64
		if h.pool != nil {
			_ = h.pool.QueryRow(r.Context(), query).Scan(&value)
		}
		return viewmodels.AdminStat{Icon: icon, Value: strconv.FormatInt(value, 10), Label: label, Delta: "Live", Positive: value == 0, Period: "current"}
	}
	switch role {
	case domainidentity.RoleFinanceOperator:
		return []viewmodels.AdminStat{
			count(`SELECT COUNT(*) FROM payment_attempts WHERE status='captured'`, "Captured payments", "payments"),
			count(`SELECT COUNT(*) FROM payment_attempts WHERE status='failed'`, "Failed payments", "error"),
			count(`SELECT COUNT(*) FROM seller_settlements WHERE status IN ('pending','approved')`, "Pending settlements", "account_balance"),
			count(`SELECT COUNT(*) FROM return_requests WHERE status='approved'`, "Approved returns", "currency_rupee"),
		}
	case domainidentity.RoleOperations:
		return []viewmodels.AdminStat{
			count(`SELECT COUNT(*) FROM orders WHERE status IN ('paid','processing')`, "Orders to fulfil", "inventory_2"),
			count(`SELECT COUNT(*) FROM orders WHERE status='shipped'`, "In transit", "local_shipping"),
			count(`SELECT COUNT(*) FROM return_requests WHERE status IN ('pending','approved','received')`, "Active returns", "undo"),
			count(`SELECT COUNT(*) FROM orders WHERE status='delivered'`, "Delivered orders", "task_alt"),
		}
	case domainidentity.RoleSuperAdmin:
		return []viewmodels.AdminStat{
			count(`SELECT COUNT(*) FROM users`, "User accounts", "groups"),
			count(`SELECT COUNT(*) FROM user_roles WHERE role_slug <> 'customer'`, "Operational role assignments", "admin_panel_settings"),
			count(`SELECT COUNT(*) FROM audit_logs`, "Recorded audit events", "history"),
			count(`SELECT COUNT(*) FROM support_tickets WHERE status IN ('open','in_progress','waiting_customer')`, "Open support cases", "support_agent"),
		}
	default:
		return []viewmodels.AdminStat{
			count(`SELECT COUNT(*) FROM products WHERE status='pending_review'`, "Products awaiting review", "fact_check"),
			count(`SELECT COUNT(*) FROM sellers WHERE status='pending'`, "Seller applications", "groups"),
			count(`SELECT COUNT(*) FROM orders`, "Orders", "shopping_bag"),
			count(`SELECT COUNT(*) FROM return_requests WHERE status='pending'`, "Pending returns", "undo"),
		}
	}
}

func (h *LivePortalHandler) adminInventory(r *http.Request) []viewmodels.AdminInventory {
	if h.pool == nil {
		return nil
	}
	rows, err := h.pool.Query(r.Context(), `
		SELECT p.name, pv.sku, COALESCE(stock.available_quantity, 0), COALESCE(stock.reserved_quantity, 0), pv.price_cents
		FROM product_variants pv
		JOIN products p ON p.id = pv.product_id
		LEFT JOIN inventory_stock stock ON stock.variant_id = pv.id
		WHERE pv.status = 'active'
		ORDER BY p.name, pv.id
		LIMIT 100`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	items := make([]viewmodels.AdminInventory, 0)
	for rows.Next() {
		var name, sku string
		var available, reserved int64
		var price int64
		if rows.Scan(&name, &sku, &available, &reserved, &price) != nil {
			continue
		}
		onHand := available + reserved
		status := "in_stock"
		if available <= 5 {
			status = "low_stock"
		}
		items = append(items, viewmodels.AdminInventory{
			Name: name, SKU: sku, Stock: strconv.FormatInt(onHand, 10), Reorder: "5",
			Reserved: strconv.FormatInt(reserved, 10), Available: strconv.FormatInt(available, 10),
			Location: "India", Value: money(price * available), Status: status,
			Image: "/assets/images/customer/madhubani-tree.webp",
		})
	}
	return items
}

func (h *LivePortalHandler) adminSellers(r *http.Request) []viewmodels.AdminSeller {
	if h.pool == nil {
		return nil
	}
	rows, err := h.pool.Query(r.Context(), `SELECT id, display_name, status, created_at FROM sellers ORDER BY id DESC LIMIT 50`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var result []viewmodels.AdminSeller
	for rows.Next() {
		var id int64
		var name, status string
		var created time.Time
		if rows.Scan(&id, &name, &status, &created) == nil {
			result = append(result, viewmodels.AdminSeller{ID: strconv.FormatInt(id, 10), Name: name, Status: status, Applied: created.Format("2 Jan 2006"), Image: "/assets/images/customer/maker-mithila.webp"})
		}
	}
	return result
}

func (h *LivePortalHandler) adminProducts(r *http.Request, query string) []viewmodels.AdminProduct {
	rows, err := h.pool.Query(r.Context(), `SELECT p.id,p.name, COALESCE(pv.sku,''), COALESCE(s.display_name,''), c.name, p.price_cents, p.status, p.image_url,p.created_at FROM products p JOIN categories c ON c.slug=p.category_slug LEFT JOIN sellers s ON s.id=p.seller_id LEFT JOIN LATERAL (SELECT sku,price_cents FROM product_variants WHERE product_id=p.id ORDER BY id LIMIT 1) pv ON TRUE WHERE ($1='' OR p.name ILIKE '%'||$1||'%' OR p.slug ILIKE '%'||$1||'%' OR COALESCE(pv.sku,'') ILIKE '%'||$1||'%' OR COALESCE(s.display_name,'') ILIKE '%'||$1||'%' OR c.name ILIKE '%'||$1||'%' OR p.status ILIKE '%'||$1||'%') ORDER BY p.created_at DESC,p.id DESC LIMIT 100`, query)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var result []viewmodels.AdminProduct
	for rows.Next() {
		var id int64
		var name, sku, seller, category, status, image string
		var submitted time.Time
		var price int64
		if rows.Scan(&id, &name, &sku, &seller, &category, &price, &status, &image, &submitted) == nil {
			result = append(result, viewmodels.AdminProduct{ID: strconv.FormatInt(id, 10), Name: name, SKU: sku, Seller: seller, Category: category, Price: money(price), Status: status, Submitted: submitted.Format("2 Jan 2006"), Image: image})
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
			result = append(result, viewmodels.AdminReturn{ID: strconv.FormatInt(id, 10), Number: fmt.Sprintf("RTN-%d", id), Customer: customer, Product: number, Reason: reason, Status: status, Amount: money(amount), Image: "/assets/images/customer/terracotta-lamp.webp"})
		}
	}
	return result
}

func (h *LivePortalHandler) adminTransactions(r *http.Request) []viewmodels.AdminTransaction {
	rows, err := h.pool.Query(r.Context(), `SELECT pa.created_at, pa.status, o.order_number, pa.amount_cents, pa.provider FROM payment_attempts pa JOIN orders o ON o.id=pa.order_id ORDER BY pa.created_at DESC LIMIT 50`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var result []viewmodels.AdminTransaction
	for rows.Next() {
		var created time.Time
		var status, reference, provider string
		var amount int64
		if rows.Scan(&created, &status, &reference, &amount, &provider) == nil {
			result = append(result, viewmodels.AdminTransaction{Date: created.Format("2 Jan 2006"), Type: "Payment", Reference: reference, Description: provider, Amount: money(amount), Status: status})
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

func (h *LivePortalHandler) renderSeller(w http.ResponseWriter, r *http.Request, userID int64, displayName string, isOwner bool, permissions []string, csrfToken string) {
	page := h.sellerPage(r, userID, isOwner, permissions)
	page.DisplayName = displayName
	page.UserEmail = h.userEmail(r, userID)
	page.Initials = initials(displayName)
	page.CSRFToken = csrfToken
	page.RoleLabel = "Seller Staff"
	if isOwner {
		page.RoleLabel = "Seller Owner"
	}
	page.CanProducts = sellerPortalAllowed("/seller-admin/products", isOwner, permissions)
	page.CanProductWrite = isOwner || containsString(permissions, "PRODUCT_WRITE")
	page.CanInventory = sellerPortalAllowed("/seller-admin/inventory", isOwner, permissions)
	page.CanInventoryWrite = isOwner || containsString(permissions, "INVENTORY_WRITE")
	page.CanOrders = sellerPortalAllowed("/seller-admin/orders", isOwner, permissions)
	page.CanFulfill = isOwner || containsString(permissions, "ORDER_FULFILL")
	page.CanReturns = sellerPortalAllowed("/seller-admin/returns", isOwner, permissions)
	page.CanReports = sellerPortalAllowed("/seller-admin/analytics", isOwner, permissions)
	page.CanTeam = isOwner
	var component templ.Component
	switch page.Active {
	case "dashboard":
		component = sellerpages.PortalDashboard(page)
	case "onboarding":
		component = sellerpages.PortalOnboarding(page)
	case "products":
		component = sellerpages.PortalProducts(page)
	case "inventory":
		component = sellerpages.PortalInventory(page)
	case "orders":
		component = sellerpages.PortalOrders(page)
	case "returns":
		component = sellerpages.PortalReturns(page)
	case "earnings":
		component = sellerpages.PortalEarnings(page)
	case "analytics":
		component = sellerpages.PortalAnalytics(page)
	case "settings":
		component = sellerpages.PortalSettings(page)
	case "team":
		component = sellerpages.PortalTeam(page)
	default:
		component = sellerpages.PortalUnavailable(page)
	}
	h.render(w, r, component)
}

func isSellerProductFormPath(path string) bool {
	if path == "/seller-admin/products/new" {
		return true
	}
	return isSellerProductEditPath(strings.Trim(strings.TrimPrefix(path, "/seller-admin/"), "/"))
}

func (h *LivePortalHandler) renderSellerProductForm(w http.ResponseWriter, r *http.Request, userID int64, displayName string, isOwner bool, permissions []string, csrfToken string) {
	pageRequest := requestWithPath(r, "/seller-admin/products")
	page := h.sellerPage(pageRequest, userID, isOwner, permissions)
	page.DisplayName = displayName
	page.UserEmail = h.userEmail(r, userID)
	page.Initials = initials(displayName)
	page.CSRFToken = csrfToken
	page.RoleLabel = roleLabel(domainidentity.RoleSellerStaff)
	if isOwner {
		page.RoleLabel = roleLabel(domainidentity.RoleSellerOwner)
	}
	page.CanProducts = true
	page.CanProductWrite = true
	page.CanInventoryWrite = isOwner || containsString(permissions, "INVENTORY_WRITE")
	page.CanInventory = sellerPortalAllowed("/seller-admin/inventory", isOwner, permissions)
	page.CanOrders = sellerPortalAllowed("/seller-admin/orders", isOwner, permissions)
	page.CanFulfill = isOwner || containsString(permissions, "ORDER_FULFILL")
	page.CanReturns = sellerPortalAllowed("/seller-admin/returns", isOwner, permissions)
	page.CanReports = sellerPortalAllowed("/seller-admin/analytics", isOwner, permissions)
	page.CanTeam = isOwner

	form := viewmodels.SellerProductForm{Action: "/seller-admin/products", InitialStock: "0", DeliveryLabel: "Delivery to serviceable PIN codes in India"}
	if r.URL.Path != "/seller-admin/products/new" {
		relative := strings.Trim(strings.TrimPrefix(r.URL.Path, "/seller-admin/"), "/")
		idText := strings.TrimSuffix(strings.TrimPrefix(relative, "products/"), "/edit")
		id, err := strconv.ParseInt(idText, 10, 64)
		if err != nil || id <= 0 {
			http.NotFound(w, r)
			return
		}
		var price, compareAt, stock int64
		var hasCompareAt bool
		err = h.pool.QueryRow(r.Context(), `
			SELECT p.id::text,p.name,p.slug,p.brand_slug,b.name,p.category_slug,c.name,p.description,p.price_cents,COALESCE(p.compare_at_cents,0),p.compare_at_cents IS NOT NULL,p.image_url,
			       COALESCE(v.sku,''),COALESCE(i.available_quantity,0),p.delivery_label
			FROM products p JOIN brands b ON b.slug=p.brand_slug JOIN categories c ON c.slug=p.category_slug
			LEFT JOIN LATERAL (SELECT id,sku FROM product_variants WHERE product_id=p.id AND status='active' ORDER BY id LIMIT 1) v ON TRUE
			LEFT JOIN inventory_stock i ON i.variant_id=v.id
			WHERE p.id=$1 AND p.status<>'archived' AND p.seller_id IN (
				SELECT s.id FROM sellers s WHERE s.owner_user_id=$2 OR EXISTS (
					SELECT 1 FROM seller_users su WHERE su.seller_id=s.id AND su.user_id=$2 AND su.status='active'))`, id, userID).Scan(&form.ID, &form.Name, &form.Slug, &form.BrandSlug, &form.BrandName, &form.CategorySlug, &form.CategoryName, &form.Description, &price, &compareAt, &hasCompareAt, &form.Image, &form.SKU, &stock, &form.DeliveryLabel)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		form.PriceCents = strconv.FormatInt(price, 10)
		if hasCompareAt {
			form.CompareAtCents = strconv.FormatInt(compareAt, 10)
		}
		form.InitialStock = strconv.FormatInt(stock, 10)
		form.Action = "/seller-admin/products/" + idText + "/edit"
		form.Variants, err = h.productVariantViews(r, id)
		if err != nil {
			http.Error(w, "Could not load product variants", http.StatusServiceUnavailable)
			return
		}
		form.Media, err = productMediaViews(r.Context(), h.pool, id)
		if err != nil {
			http.Error(w, "Could not load product media", http.StatusServiceUnavailable)
			return
		}
		page.Title = "Edit product draft"
	} else {
		page.Title = "Create product draft"
	}
	page.Subtitle = "Save a product draft, then submit it for marketplace review. Products ship to serviceable PIN codes in India."
	h.render(w, r, sellerpages.ProductForm(page, form))
}

func productMediaViews(ctx context.Context, pool *pgxpool.Pool, productID int64) ([]viewmodels.ProductMedia, error) {
	rows, err := pool.Query(ctx, `SELECT id,public_url,content_type,byte_size FROM product_media WHERE product_id=$1 AND status='ready' AND visibility='public' ORDER BY created_at,id`, productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	media := make([]viewmodels.ProductMedia, 0)
	for rows.Next() {
		var id, byteSize int64
		var path, contentType string
		if err := rows.Scan(&id, &path, &contentType, &byteSize); err != nil {
			return nil, err
		}
		media = append(media, viewmodels.ProductMedia{ID: strconv.FormatInt(id, 10), URL: path, ContentType: contentType, Size: strconv.FormatInt(byteSize, 10), IsVideo: strings.HasPrefix(contentType, "video/")})
	}
	return media, rows.Err()
}

func (h *LivePortalHandler) productVariantViews(r *http.Request, productID int64) ([]viewmodels.ProductVariant, error) {
	rows, err := h.pool.Query(r.Context(), `SELECT pv.id,pv.name,pv.sku,pv.price_cents,COALESCE(pv.compare_at_cents,0),
		pv.compare_at_cents IS NOT NULL,COALESCE(i.available_quantity,0),COALESCE(i.reserved_quantity,0),pv.status
		FROM product_variants pv LEFT JOIN inventory_stock i ON i.variant_id=pv.id
		WHERE pv.product_id=$1 ORDER BY CASE WHEN pv.status='active' THEN 0 ELSE 1 END,pv.id`, productID)
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
		variant := viewmodels.ProductVariant{ID: strconv.FormatInt(id, 10), Name: name, SKU: sku, PriceCents: strconv.FormatInt(price, 10),
			Available: strconv.FormatInt(available, 10), Reserved: strconv.FormatInt(reserved, 10), Status: status, HasCompareAt: hasCompare}
		if hasCompare {
			variant.CompareAtCents = strconv.FormatInt(compareAt, 10)
		}
		variants = append(variants, variant)
	}
	return variants, rows.Err()
}

func (h *LivePortalHandler) sellerPage(r *http.Request, userID int64, isOwner bool, permissions []string) viewmodels.SellerAdminPage {
	path := strings.TrimPrefix(r.URL.Path, "/seller-admin")
	active := strings.Trim(path, "/")
	if active == "" {
		active = "dashboard"
	}
	var sellerID int64
	var name, sellerStatus string
	if err := h.pool.QueryRow(r.Context(), `SELECT s.id,s.display_name,s.status FROM sellers s WHERE s.owner_user_id=$1 OR EXISTS (SELECT 1 FROM seller_users su WHERE su.seller_id=s.id AND su.user_id=$1 AND su.status='active') ORDER BY CASE WHEN s.owner_user_id=$1 THEN 0 ELSE 1 END,s.id LIMIT 1`, userID).Scan(&sellerID, &name, &sellerStatus); err != nil {
		return viewmodels.SellerAdminPage{Active: active, Title: "Seller workspace", Subtitle: "No seller profile is assigned to this account."}
	}
	page := viewmodels.SellerAdminPage{Active: active, Workspace: active, Title: title(active), Subtitle: name + " · seller workspace", Query: r.URL.Query().Get("q"), Tab: r.URL.Query().Get("tab"), Range: r.URL.Query().Get("range"), Notice: r.URL.Query().Get("notice"), SellerName: name, SellerStatus: sellerStatus}
	if sellerPortalAllowed("/seller-admin/products", isOwner, permissions) {
		page.Products = h.sellerProducts(r, sellerID, portalQuery(page.Query))
	}
	if sellerPortalAllowed("/seller-admin/inventory", isOwner, permissions) {
		page.Inventory = h.sellerInventory(r, sellerID)
	}
	if sellerPortalAllowed("/seller-admin/orders", isOwner, permissions) {
		page.Orders = h.sellerOrders(r, sellerID)
	}
	if sellerPortalAllowed("/seller-admin/returns", isOwner, permissions) {
		page.Returns = h.sellerReturns(r, sellerID)
	}
	if sellerPortalAllowed("/seller-admin/analytics", isOwner, permissions) {
		page.Stats = h.sellerStats(r, sellerID)
	}
	if sellerPortalAllowed("/seller-admin/analytics", isOwner, permissions) {
		page.Ledger = h.sellerLedger(r, sellerID)
		page.Settlements = h.sellerSettlements(r, sellerID)
	}
	if isOwner {
		page.Team = h.sellerTeam(r, sellerID)
	}
	filterSellerLivePage(&page)
	return page
}

func filterSellerLivePage(page *viewmodels.SellerAdminPage) {
	query := strings.ToLower(strings.TrimSpace(page.Query))
	if query != "" {
		filteredProducts := page.Products[:0]
		for _, item := range page.Products {
			if strings.Contains(strings.ToLower(item.Name+" "+item.SKU+" "+item.Category+" "+item.Status), query) {
				filteredProducts = append(filteredProducts, item)
			}
		}
		page.Products = filteredProducts
		filteredInventory := page.Inventory[:0]
		for _, item := range page.Inventory {
			if strings.Contains(strings.ToLower(item.Name+" "+item.SKU+" "+item.Variant+" "+item.Status), query) {
				filteredInventory = append(filteredInventory, item)
			}
		}
		page.Inventory = filteredInventory
		filteredOrders := page.Orders[:0]
		for _, item := range page.Orders {
			if strings.Contains(strings.ToLower(item.Number+" "+item.Customer+" "+item.Status+" "+item.Payment), query) {
				filteredOrders = append(filteredOrders, item)
			}
		}
		page.Orders = filteredOrders
		filteredReturns := page.Returns[:0]
		for _, item := range page.Returns {
			if strings.Contains(strings.ToLower(item.Number+" "+item.Order+" "+item.Customer+" "+item.Reason+" "+item.Status), query) {
				filteredReturns = append(filteredReturns, item)
			}
		}
		page.Returns = filteredReturns
	}
	if page.Tab != "" {
		filteredProducts := page.Products[:0]
		for _, item := range page.Products {
			if sellerTabMatches(page.Tab, item.Status, item.Stock) {
				filteredProducts = append(filteredProducts, item)
			}
		}
		page.Products = filteredProducts
		filteredOrders := page.Orders[:0]
		for _, item := range page.Orders {
			if sellerTabMatches(page.Tab, item.Status, "") {
				filteredOrders = append(filteredOrders, item)
			}
		}
		page.Orders = filteredOrders
	}
}

func (h *LivePortalHandler) sellerStats(r *http.Request, sellerID int64) []viewmodels.SellerAdminStat {
	var products, orders, stock int64
	_ = h.pool.QueryRow(r.Context(), `SELECT COUNT(*) FROM products WHERE seller_id=$1`, sellerID).Scan(&products)
	_ = h.pool.QueryRow(r.Context(), `SELECT COUNT(DISTINCT o.id) FROM orders o JOIN order_items oi ON oi.order_id=o.id JOIN product_variants v ON v.id=oi.variant_id JOIN products p ON p.id=v.product_id WHERE p.seller_id=$1 AND o.status IN ('paid','processing','shipped','delivered')`, sellerID).Scan(&orders)
	_ = h.pool.QueryRow(r.Context(), `SELECT COALESCE(SUM(i.available_quantity),0) FROM inventory_stock i JOIN product_variants v ON v.id=i.variant_id JOIN products p ON p.id=v.product_id WHERE p.seller_id=$1`, sellerID).Scan(&stock)
	return []viewmodels.SellerAdminStat{{Icon: "category", Value: strconv.FormatInt(products, 10), Label: "Products", Delta: "Live", Positive: true, Period: "database", Tone: "green"}, {Icon: "inventory_2", Value: strconv.FormatInt(orders, 10), Label: "Orders", Delta: "Live", Positive: true, Period: "database", Tone: "blue"}, {Icon: "warehouse", Value: strconv.FormatInt(stock, 10), Label: "Available stock", Delta: "Live", Positive: true, Period: "database", Tone: "orange"}}
}

func (h *LivePortalHandler) sellerProducts(r *http.Request, sellerID int64, query string) []viewmodels.SellerAdminProduct {
	rows, err := h.pool.Query(r.Context(), `SELECT p.id,p.name, COALESCE(v.sku,''), c.name, p.price_cents, p.status, COALESCE(i.available_quantity,0), (SELECT COUNT(*) FROM product_variants pv WHERE pv.product_id=p.id AND pv.status='active'),p.image_url FROM products p JOIN categories c ON c.slug=p.category_slug LEFT JOIN LATERAL (SELECT id,sku,price_cents FROM product_variants WHERE product_id=p.id ORDER BY id LIMIT 1) v ON TRUE LEFT JOIN inventory_stock i ON i.variant_id=v.id WHERE p.seller_id=$1 AND ($2='' OR p.name ILIKE '%'||$2||'%' OR p.slug ILIKE '%'||$2||'%' OR COALESCE(v.sku,'') ILIKE '%'||$2||'%' OR c.name ILIKE '%'||$2||'%' OR p.status ILIKE '%'||$2||'%') ORDER BY p.created_at DESC,p.id DESC LIMIT 100`, sellerID, query)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var result []viewmodels.SellerAdminProduct
	for rows.Next() {
		var id int64
		var name, sku, category, status, image string
		var price, stock int64
		var variants int
		if rows.Scan(&id, &name, &sku, &category, &price, &status, &stock, &variants, &image) == nil {
			result = append(result, viewmodels.SellerAdminProduct{ID: strconv.FormatInt(id, 10), Name: name, SKU: sku, Category: category, Price: money(price), Stock: strconv.FormatInt(stock, 10), Status: status, Variants: strconv.Itoa(variants), Availability: "D", Image: image})
		}
	}
	return result
}

func (h *LivePortalHandler) sellerInventory(r *http.Request, sellerID int64) []viewmodels.SellerAdminInventory {
	rows, err := h.pool.Query(r.Context(), `SELECT v.id,p.name,v.sku,COALESCE(v.name,'Default variant'),COALESCE(i.available_quantity,0),COALESCE(i.reserved_quantity,0) FROM products p JOIN product_variants v ON v.product_id=p.id LEFT JOIN inventory_stock i ON i.variant_id=v.id WHERE p.seller_id=$1 ORDER BY p.id,v.id LIMIT 100`, sellerID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var result []viewmodels.SellerAdminInventory
	for rows.Next() {
		var variantID int64
		var name, sku, variant string
		var available, reserved int64
		if rows.Scan(&variantID, &name, &sku, &variant, &available, &reserved) == nil {
			status := "In stock"
			if available == 0 {
				status = "Out of stock"
			} else if available <= 5 {
				status = "Low stock"
			}
			result = append(result, viewmodels.SellerAdminInventory{VariantID: strconv.FormatInt(variantID, 10), Name: name, SKU: sku, Variant: variant, Location: "India", Available: strconv.FormatInt(available, 10), Reserved: strconv.FormatInt(reserved, 10), Reorder: "5", Status: status, Image: "/assets/images/customer/madhubani-tree.webp"})
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
			result = append(result, viewmodels.SellerAdminOrder{SellerID: strconv.FormatInt(sellerID, 10), Number: number, Date: created.Format("2 Jan 2006"), Customer: customer, Items: strconv.FormatInt(items, 10), Value: money(total), Type: "Domestic", Destination: "India", Status: status, Payment: displayPaymentStatus(payment, ""), Image: "/assets/images/customer/madhubani-tree.webp"})
		}
	}
	return result
}

func (h *LivePortalHandler) sellerReturns(r *http.Request, sellerID int64) []viewmodels.SellerAdminReturn {
	rows, err := h.pool.Query(r.Context(), `
		SELECT rr.id,o.order_number,u.display_name,rr.reason,rr.created_at,rr.status,o.total_cents,
		       COALESCE(string_agg(DISTINCT p.name, ', '),'')
		FROM return_requests rr
		JOIN orders o ON o.id=rr.order_id
		JOIN users u ON u.id=rr.user_id
		JOIN order_items oi ON oi.order_id=o.id
		JOIN product_variants v ON v.id=oi.variant_id
		JOIN products p ON p.id=v.product_id AND p.seller_id=$1
		GROUP BY rr.id,o.order_number,u.display_name,rr.reason,rr.created_at,rr.status,o.total_cents
		ORDER BY rr.created_at DESC LIMIT 50`, sellerID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var result []viewmodels.SellerAdminReturn
	for rows.Next() {
		var id int64
		var order, customer, reason, status, product string
		var created time.Time
		var amount int64
		if rows.Scan(&id, &order, &customer, &reason, &created, &status, &amount, &product) == nil {
			result = append(result, viewmodels.SellerAdminReturn{Number: fmt.Sprintf("RTN-%d", id), Order: order, Customer: maskName(customer), Product: product, Reason: reason, Date: created.Format("2 Jan 2006"), Status: status, Amount: money(amount), Image: "/assets/images/customer/madhubani-tree.webp"})
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
	rows, err := h.pool.Query(r.Context(), `SELECT u.id,u.display_name,u.email,su.status,su.permissions FROM seller_users su JOIN users u ON u.id=su.user_id AND u.status='active' WHERE su.seller_id=$1 AND su.status='active' ORDER BY u.id`, sellerID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var result []viewmodels.SellerTeamMember
	for rows.Next() {
		var userID int64
		var name, email, status string
		var permissions []string
		if rows.Scan(&userID, &name, &email, &status, &permissions) == nil {
			result = append(result, viewmodels.SellerTeamMember{UserID: strconv.FormatInt(userID, 10), Name: name, Email: email, Role: "Seller staff", Status: status, Permissions: strings.Join(permissions, ", "), LastActive: "Live account", ProductRead: sellerHasPermission(permissions, "PRODUCT_READ"), ProductWrite: sellerHasPermission(permissions, "PRODUCT_WRITE"), InventoryRead: sellerHasPermission(permissions, "INVENTORY_READ"), InventoryWrite: sellerHasPermission(permissions, "INVENTORY_WRITE"), OrderRead: sellerHasPermission(permissions, "ORDER_READ"), OrderFulfill: sellerHasPermission(permissions, "ORDER_FULFILL"), ReturnRead: sellerHasPermission(permissions, "RETURN_READ"), ReportRead: sellerHasPermission(permissions, "REPORT_READ")})
		}
	}
	return result
}

func portalQuery(value string) string {
	value = strings.TrimSpace(value)
	runes := []rune(value)
	if len(runes) > 120 {
		return string(runes[:120])
	}
	return value
}

func sellerHasPermission(permissions []string, wanted string) bool {
	for _, permission := range permissions {
		if permission == wanted {
			return true
		}
	}
	return false
}

func (h *LivePortalHandler) renderSupport(w http.ResponseWriter, r *http.Request, actorID int64, csrfToken string) {
	page := h.supportPage(r, actorID, csrfToken)
	var component templ.Component
	switch {
	case strings.HasSuffix(r.URL.Path, "/customers"):
		component = supportpages.LiveCustomers(page)
	case strings.HasSuffix(r.URL.Path, "/orders"):
		component = supportpages.LiveOrders(page)
	case strings.HasSuffix(r.URL.Path, "/cases"):
		component = supportpages.LiveCases(page)
	default:
		component = supportpages.LiveDashboard(page)
	}
	h.render(w, r, component)
}

func (h *LivePortalHandler) supportPage(r *http.Request, actorID int64, csrfToken string) viewmodels.SupportPortalPage {
	active := "dashboard"
	trimmed := strings.Trim(strings.TrimPrefix(r.URL.Path, "/support-portal"), "/")
	if trimmed != "" {
		active = trimmed
	}
	page := viewmodels.SupportPortalPage{Active: active, Title: title(active), Subtitle: "Support cases backed by live PostgreSQL records.", Query: r.URL.Query().Get("q"), Notice: r.URL.Query().Get("notice"), Tab: r.URL.Query().Get("tab"), DisplayName: h.userDisplayName(r, actorID), RoleLabel: roleLabel(domainidentity.RoleSupportAgent), CSRFToken: csrfToken}
	page.Initials = initials(page.DisplayName)
	page.Cases = h.supportCases(r)
	if query := strings.ToLower(strings.TrimSpace(page.Query)); query != "" {
		page.Cases = filterSupportCases(page.Cases, query)
	}
	page.Conversations = h.latestSupportConversations(r)
	if strings.EqualFold(active, "cases") && len(page.Cases) > 0 {
		selectedID, _ := strconv.ParseInt(r.URL.Query().Get("ticket_id"), 10, 64)
		for _, item := range page.Cases {
			if selectedID == 0 || item.DatabaseID == selectedID {
				page.SelectedCase = item
				break
			}
		}
		if page.SelectedCase.DatabaseID > 0 {
			page.Messages = append(page.Messages, h.supportTicketMessages(r, page.SelectedCase.DatabaseID)...)
		}
	}
	if strings.EqualFold(active, "orders") {
		linked := make([]viewmodels.SupportPortalCase, 0, len(page.Cases))
		for _, item := range page.Cases {
			if item.Order != "" {
				linked = append(linked, item)
			}
		}
		page.Cases = linked
	}
	if strings.EqualFold(page.Active, "customers") {
		page.Directory = supportDirectoryFromCases(page.Cases)
	}
	open, urgent := 0, 0
	for _, item := range page.Cases {
		if item.Status != "resolved" && item.Status != "closed" {
			open++
		}
		if item.Priority == "urgent" {
			urgent++
		}
	}
	page.Stats = []viewmodels.SupportPortalStat{{Icon: "support_agent", Value: strconv.Itoa(open), Label: "Open tickets", Delta: "Live", Positive: true, Period: "PostgreSQL", Tone: "green"}, {Icon: "warning", Value: strconv.Itoa(urgent), Label: "Urgent tickets", Delta: "Live", Period: "PostgreSQL", Tone: "red"}, {Icon: "task_alt", Value: strconv.Itoa(len(page.Cases) - open), Label: "Resolved or closed", Delta: "Live", Period: "PostgreSQL", Tone: "blue"}}
	switch active {
	case "dashboard":
		page.Title = "Support dashboard"
	case "cases":
		page.Title = "Support cases"
	case "customers":
		page.Title = "Customers in support cases"
	case "orders":
		page.Title = "Orders linked to cases"
	}
	return page
}

func (h *LivePortalHandler) supportCases(r *http.Request) []viewmodels.SupportPortalCase {
	rows, err := h.pool.Query(r.Context(), `SELECT t.id,t.ticket_number,t.subject,t.user_id,u.display_name,t.priority,t.created_at,t.status,t.message,t.agent_note,COALESCE(o.order_number,''),COALESCE(o.status,''),COALESCE(o.total_cents,0),COALESCE(pa.status,''),COALESCE(reply.body,'') FROM support_tickets t JOIN users u ON u.id=t.user_id LEFT JOIN orders o ON o.id=t.order_id LEFT JOIN LATERAL (SELECT status FROM payment_attempts WHERE order_id=o.id ORDER BY id DESC LIMIT 1) pa ON TRUE LEFT JOIN LATERAL (SELECT body FROM support_ticket_messages WHERE ticket_id=t.id AND author_role='support_agent' ORDER BY created_at DESC,id DESC LIMIT 1) reply ON TRUE ORDER BY t.created_at DESC LIMIT 200`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var result []viewmodels.SupportPortalCase
	for rows.Next() {
		var id, customerID int64
		var number, subject, customer, priority, status, initialMessage, agentNote, orderNumber, orderStatus, paymentStatus, latestReply string
		var orderTotal int64
		var created time.Time
		if rows.Scan(&id, &number, &subject, &customerID, &customer, &priority, &created, &status, &initialMessage, &agentNote, &orderNumber, &orderStatus, &orderTotal, &paymentStatus, &latestReply) == nil {
			result = append(result, viewmodels.SupportPortalCase{DatabaseID: id, CustomerID: customerID, ID: number, Subject: subject, Customer: maskName(customer), CustomerInitials: initials(customer), Category: "Support request", Priority: priority, Created: created.Format("2 Jan 2006 15:04"), Status: status, InitialMessage: initialMessage, AgentNote: agentNote, Order: orderNumber, OrderStatus: orderStatus, OrderValue: money(orderTotal), PaymentStatus: paymentStatus, LatestReply: latestReply})
		}
	}
	return result
}

func (h *LivePortalHandler) supportTicketMessages(r *http.Request, ticketID int64) []viewmodels.SupportPortalConversation {
	rows, err := h.pool.Query(r.Context(), `SELECT m.author_id,u.display_name,m.author_role,m.body,m.created_at FROM support_ticket_messages m JOIN users u ON u.id=m.author_id WHERE m.ticket_id=$1 ORDER BY m.created_at,m.id`, ticketID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var messages []viewmodels.SupportPortalConversation
	for rows.Next() {
		var authorID int64
		var author, authorRole, body string
		var created time.Time
		if rows.Scan(&authorID, &author, &authorRole, &body, &created) == nil {
			role, tone := "Customer", "customer"
			if authorRole == "support_agent" {
				role, tone = "Support Agent", "agent"
			}
			messages = append(messages, viewmodels.SupportPortalConversation{TicketID: ticketID, Author: author, Initials: initials(author), Role: role, Message: body, Time: created.Format("2 Jan 2006 15:04"), Tone: tone})
		}
	}
	return messages
}

func (h *LivePortalHandler) latestSupportConversations(r *http.Request) []viewmodels.SupportPortalConversation {
	rows, err := h.pool.Query(r.Context(), `SELECT m.ticket_id,t.ticket_number,u.display_name,m.body,m.created_at FROM support_ticket_messages m JOIN support_tickets t ON t.id=m.ticket_id JOIN users u ON u.id=m.author_id WHERE m.author_role='support_agent' ORDER BY m.created_at DESC,m.id DESC LIMIT 12`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var messages []viewmodels.SupportPortalConversation
	for rows.Next() {
		var item viewmodels.SupportPortalConversation
		var created time.Time
		if rows.Scan(&item.TicketID, &item.TicketNumber, &item.Author, &item.Message, &created) == nil {
			item.Initials = initials(item.Author)
			item.Role = "Support Agent"
			item.Time = created.Format("2 Jan 2006 15:04")
			item.Tone = "agent"
			if runes := []rune(item.Message); len(runes) > 120 {
				item.Message = string(runes[:117]) + "…"
			}
			messages = append(messages, item)
		}
	}
	return messages
}

func filterSupportCases(cases []viewmodels.SupportPortalCase, query string) []viewmodels.SupportPortalCase {
	result := make([]viewmodels.SupportPortalCase, 0, len(cases))
	for _, item := range cases {
		if strings.Contains(strings.ToLower(item.ID+" "+item.Subject+" "+item.Customer+" "+item.Order+" "+item.Status), query) {
			result = append(result, item)
		}
	}
	return result
}

func supportDirectoryFromCases(cases []viewmodels.SupportPortalCase) []viewmodels.SupportPortalDirectoryEntry {
	type person struct {
		entry viewmodels.SupportPortalDirectoryEntry
		seen  map[string]struct{}
	}
	byUser := make(map[int64]*person)
	for _, item := range cases {
		current := byUser[item.CustomerID]
		if current == nil {
			current = &person{entry: viewmodels.SupportPortalDirectoryEntry{Name: item.Customer, Type: "Customer", EmailPhone: "Contact details masked", Status: "Has support ticket"}, seen: make(map[string]struct{})}
			byUser[item.CustomerID] = current
		}
		current.seen[item.ID] = struct{}{}
	}
	result := make([]viewmodels.SupportPortalDirectoryEntry, 0, len(byUser))
	for _, current := range byUser {
		current.entry.Cases = strconv.Itoa(len(current.seen))
		result = append(result, current.entry)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}

func (h *LivePortalHandler) render(w http.ResponseWriter, r *http.Request, component templ.Component) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := component.Render(r.Context(), w); err != nil {
		http.Error(w, "portal rendering failed", http.StatusInternalServerError)
	}
}

func (h *LivePortalHandler) renderAdminComponent(w http.ResponseWriter, r *http.Request, component templ.Component, page viewmodels.AdminPage) {
	var body bytes.Buffer
	if err := component.Render(r.Context(), &body); err != nil {
		http.Error(w, "portal rendering failed", http.StatusInternalServerError)
		return
	}
	financeTarget := page.PortalRoot + "/finance"
	if !page.CanFinance {
		financeTarget = "/workspaces"
	}
	replacer := strings.NewReplacer(
		`href="/admin`, `href="`+page.PortalRoot,
		`action="/admin`, `action="`+page.PortalRoot,
		`href="/finance`, `href="`+financeTarget,
		`action="/finance`, `action="`+financeTarget,
	)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(replacer.Replace(body.String())))
}

func containsString(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func filterAdminPage(page *viewmodels.AdminPage, query string) {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return
	}
	contains := func(values ...string) bool {
		for _, value := range values {
			if strings.Contains(strings.ToLower(value), query) {
				return true
			}
		}
		return false
	}
	products := page.Products[:0]
	for _, item := range page.Products {
		if contains(item.Name, item.SKU, item.Seller, item.Category, item.Status) {
			products = append(products, item)
		}
	}
	page.Products = products
	sellers := page.Sellers[:0]
	for _, item := range page.Sellers {
		if contains(item.Name, item.Status) {
			sellers = append(sellers, item)
		}
	}
	page.Sellers = sellers
	orders := page.Orders[:0]
	for _, item := range page.Orders {
		if contains(item.Number, item.Customer, item.Fulfillment) {
			orders = append(orders, item)
		}
	}
	page.Orders = orders
	returns := page.Returns[:0]
	for _, item := range page.Returns {
		if contains(item.Number, item.Customer, item.Product, item.Reason, item.Status) {
			returns = append(returns, item)
		}
	}
	page.Returns = returns
	cases := page.SupportCases[:0]
	for _, item := range page.SupportCases {
		if contains(item.Case, item.Customer, item.Subject, item.Status, item.Priority) {
			cases = append(cases, item)
		}
	}
	page.SupportCases = cases
	transactions := page.Transactions[:0]
	for _, item := range page.Transactions {
		if contains(item.Reference, item.Type, item.Description, item.Status) {
			transactions = append(transactions, item)
		}
	}
	page.Transactions = transactions
	settlements := page.Settlements[:0]
	for _, item := range page.Settlements {
		if contains(item.Seller, item.Status, item.Date) {
			settlements = append(settlements, item)
		}
	}
	page.Settlements = settlements
}

func keepAdminTransactions(items []viewmodels.AdminTransaction, status string) []viewmodels.AdminTransaction {
	filtered := make([]viewmodels.AdminTransaction, 0, len(items))
	for _, item := range items {
		if strings.EqualFold(item.Status, status) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func (h *LivePortalHandler) delegate(w http.ResponseWriter, r *http.Request) {
	if h.fallback != nil {
		h.fallback.ServeHTTP(w, r)
		return
	}
	http.NotFound(w, r)
}

func (h *LivePortalHandler) delegateWithPortalRedirect(w http.ResponseWriter, r *http.Request, root string) {
	if h.fallback == nil {
		http.NotFound(w, r)
		return
	}
	h.fallback.ServeHTTP(&portalRedirectWriter{ResponseWriter: w, root: root}, r)
}

type portalRedirectWriter struct {
	http.ResponseWriter
	root string
}

func (w *portalRedirectWriter) WriteHeader(status int) {
	location := w.Header().Get("Location")
	if location != "" {
		for _, prefix := range []string{"/seller", "/support", "/admin"} {
			if location == prefix || strings.HasPrefix(location, prefix+"/") {
				location = w.root + strings.TrimPrefix(location, prefix)
				w.Header().Set("Location", location)
				break
			}
		}
	}
	w.ResponseWriter.WriteHeader(status)
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
