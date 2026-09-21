package handlers

import (
	"bytes"
	"context"
	"crypto/subtle"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
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
		if isSettlementMutationPath(r.URL.Path) {
			h.adminSettlementMutation(w, r)
			return
		}
		if isPromotionMutationPath(r.URL.Path) {
			h.adminPromotionMutation(w, r)
			return
		}
		if r.URL.Path == "/marketplace-admin/reviews/status" || r.URL.Path == "/super-admin/reviews/status" {
			h.adminReviewStatus(w, r)
			return
		}
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
	page.SettlementRoot = root
	if role == domainidentity.RoleSuperAdmin {
		page.SettlementRoot = root + "/finance"
	}
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
		if page.Workspace == "reviews" {
			component = adminpages.Workspace(page)
		} else {
			component = adminpages.PortalSupport(page)
		}
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
	if requestedWorkspace := r.URL.Query().Get("workspace"); requestedWorkspace == "settlements" && (role == domainidentity.RoleFinanceOperator || role == domainidentity.RoleSuperAdmin) {
		workspace = requestedWorkspace
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
	if workspace == "reviews" {
		page.Title = "Review moderation"
		page.Subtitle = "Moderate verified customer reviews and publish approved feedback."
	}
	page.Stats = h.adminStats(r)
	page.Bars = []viewmodels.AdminBar{{Label: "Week 1", Value: "Live order value", Height: "35", Color: "red"}, {Label: "Week 2", Value: "Live order value", Height: "50", Color: "green"}, {Label: "Week 3", Value: "Live order value", Height: "65", Color: "peach"}, {Label: "Week 4", Value: "Live order value", Height: "80", Color: "red"}}
	page.Sellers = h.adminSellers(r)
	page.Products = h.adminProducts(r, portalQuery(query))
	page.Orders = h.adminOrders(r)
	page.Returns = h.adminReturns(r)
	page.Transactions = h.adminTransactions(r)
	page.SupportCases = h.adminCases(r)
	page.Reviews = h.adminReviews(r)
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
		SELECT p.name, pv.sku, COALESCE(stock.available_quantity, 0), COALESCE(stock.reserved_quantity, 0), pv.price_cents, COALESCE(NULLIF(p.image_url, ''), '')
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
		var name, sku, image string
		var available, reserved int64
		var price int64
		if rows.Scan(&name, &sku, &available, &reserved, &price, &image) != nil {
			continue
		}
		onHand := available + reserved
		status := "in_stock"
		if available <= 5 {
			status = "low_stock"
		}
		signal := "Healthy"
		if available <= 5 {
			signal = "Low (≤5 available)"
		}
		if image == "" {
			image = "/assets/images/customer/madhubani-tree.webp"
		}
		items = append(items, viewmodels.AdminInventory{
			Name: name, SKU: sku, Stock: strconv.FormatInt(onHand, 10), Reorder: signal,
			Reserved: strconv.FormatInt(reserved, 10), Available: strconv.FormatInt(available, 10),
			Location: "India", Value: money(price * available), Status: status,
			Image: image,
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

func (h *LivePortalHandler) adminReviews(r *http.Request) []viewmodels.AdminReview {
	rows, err := h.pool.Query(r.Context(), `
		SELECT pr.id, pr.product_id, p.name, COALESCE(s.display_name,''), u.display_name,
		       pr.title, pr.body, pr.status, pr.rating, pr.verified_purchase, pr.created_at
		FROM product_reviews pr
		JOIN products p ON p.id=pr.product_id
		JOIN users u ON u.id=pr.user_id
		LEFT JOIN sellers s ON s.id=p.seller_id
		ORDER BY CASE pr.status WHEN 'pending' THEN 0 ELSE 1 END, pr.created_at DESC, pr.id DESC
		LIMIT 100`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	result := make([]viewmodels.AdminReview, 0)
	for rows.Next() {
		var review viewmodels.AdminReview
		var id, productID int64
		var created time.Time
		if rows.Scan(&id, &productID, &review.Product, &review.Seller, &review.Customer, &review.Title, &review.Body, &review.Status, &review.Rating, &review.VerifiedPurchase, &created) == nil {
			review.ID = strconv.FormatInt(id, 10)
			review.ProductID = strconv.FormatInt(productID, 10)
			review.Date = created.Format("2 Jan 2006")
			result = append(result, review)
		}
	}
	return result
}

var promotionCodePattern = regexp.MustCompile(`^[A-Z0-9][A-Z0-9_-]{2,39}$`)
var promotionAmountPattern = regexp.MustCompile(`^[0-9]{1,8}(?:\.[0-9]{1,2})?$`)

func isPromotionMutationPath(path string) bool {
	return path == "/marketplace-admin/marketing/create" || path == "/marketplace-admin/marketing/update" || path == "/marketplace-admin/marketing/status" || path == "/marketplace-admin/marketing/delete" ||
		path == "/super-admin/marketing/create" || path == "/super-admin/marketing/update" || path == "/super-admin/marketing/status" || path == "/super-admin/marketing/delete"
}

func (h *LivePortalHandler) adminPromotionMutation(w http.ResponseWriter, r *http.Request) {
	root, requiredRole := "/marketplace-admin", "marketplace_admin"
	if strings.HasPrefix(r.URL.Path, "/super-admin/") {
		root, requiredRole = "/super-admin", "super_admin"
	}
	session, ok := h.session(r)
	if !ok {
		redirectToLogin(w, r)
		return
	}
	if r.Method != http.MethodPost || !h.hasRole(r, session.UserID, requiredRole) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid promotion request", http.StatusBadRequest)
		return
	}
	if subtle.ConstantTimeCompare([]byte(session.CSRFToken), []byte(r.Form.Get("csrf_token"))) != 1 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	action := strings.TrimPrefix(r.URL.Path, root+"/marketing/")
	id, idErr := strconv.ParseInt(strings.TrimSpace(r.Form.Get("promotion_id")), 10, 64)
	var err error
	switch action {
	case "create":
		err = h.createPromotion(r, session.UserID)
	case "update":
		if idErr != nil || id <= 0 {
			err = fmt.Errorf("invalid campaign id")
		} else {
			err = h.updatePromotion(r, id, session.UserID)
		}
	case "status":
		status := strings.TrimSpace(r.Form.Get("status"))
		if idErr != nil || id <= 0 || (status != "active" && status != "draft" && status != "archived") {
			err = fmt.Errorf("invalid campaign status")
		} else {
			result, execErr := h.pool.Exec(r.Context(), `UPDATE promotions SET status=$1,updated_at=NOW() WHERE id=$2`, status, id)
			err = execErr
			if err == nil && result.RowsAffected() == 0 {
				err = fmt.Errorf("campaign not found")
			}
		}
	case "delete":
		if idErr != nil || id <= 0 {
			err = fmt.Errorf("invalid campaign id")
		} else {
			err = h.deletePromotion(r, id)
		}
	default:
		http.NotFound(w, r)
		return
	}
	if err != nil {
		status := http.StatusUnprocessableEntity
		if strings.Contains(strings.ToLower(err.Error()), "duplicate key") || strings.Contains(err.Error(), "already used") {
			status = http.StatusConflict
		}
		http.Error(w, err.Error(), status)
		return
	}
	if idErr == nil && id > 0 && action != "create" {
		_, _ = h.pool.Exec(r.Context(), `INSERT INTO audit_logs(actor_id,action,resource_type,resource_id,request_id,metadata) VALUES ($1,$2,'promotion',$3,$4,jsonb_build_object('status',$5::text))`, strconv.FormatInt(session.UserID, 10), "promotion."+action, strconv.FormatInt(id, 10), r.Header.Get("X-Request-ID"), r.Form.Get("status"))
	} else {
		_, _ = h.pool.Exec(r.Context(), `INSERT INTO audit_logs(actor_id,action,resource_type,resource_id,request_id,metadata) VALUES ($1,'promotion.create','promotion','new',$2,'{}'::jsonb)`, strconv.FormatInt(session.UserID, 10), r.Header.Get("X-Request-ID"))
	}
	http.Redirect(w, r, root+"/marketing?notice=Promotion+saved", http.StatusSeeOther)
}

type promotionForm struct {
	name, description, code, kind, slug, start, end string
	value, limit                                    int64
}

func readPromotionForm(r *http.Request) (promotionForm, error) {
	f := promotionForm{name: strings.TrimSpace(r.Form.Get("name")), description: strings.TrimSpace(r.Form.Get("description")), code: strings.ToUpper(strings.TrimSpace(r.Form.Get("code"))), kind: strings.TrimSpace(r.Form.Get("discount_type")), start: strings.TrimSpace(r.Form.Get("starts_on")), end: strings.TrimSpace(r.Form.Get("ends_on"))}
	if len([]rune(f.name)) < 3 || len([]rune(f.name)) > 100 || len([]rune(f.description)) > 2000 || !promotionCodePattern.MatchString(f.code) {
		return f, fmt.Errorf("enter a campaign name, optional description, and valid 3-40 character coupon code")
	}
	if f.kind != "percentage" && f.kind != "fixed" && f.kind != "free_shipping" {
		return f, fmt.Errorf("choose a valid discount type")
	}
	if f.kind == "fixed" {
		amount := strings.TrimSpace(r.Form.Get("discount_value"))
		if !promotionAmountPattern.MatchString(amount) {
			return f, fmt.Errorf("enter a fixed discount in rupees")
		}
		parts := strings.SplitN(amount, ".", 2)
		whole, e := strconv.ParseInt(parts[0], 10, 64)
		if e != nil {
			return f, fmt.Errorf("fixed discount is too large")
		}
		fraction := "00"
		if len(parts) == 2 {
			fraction = parts[1] + "00"
			fraction = fraction[:2]
		}
		f.value = whole * 100
		paise, _ := strconv.ParseInt(fraction, 10, 64)
		f.value += paise
		if f.value <= 0 {
			return f, fmt.Errorf("fixed discount must be greater than zero")
		}
	} else if f.kind == "percentage" {
		value, e := strconv.ParseInt(strings.TrimSpace(r.Form.Get("discount_value")), 10, 64)
		if e != nil || value < 1 || value > 100 {
			return f, fmt.Errorf("percentage discount must be from 1 to 100")
		}
		f.value = value
	}
	limitText := strings.TrimSpace(r.Form.Get("usage_limit"))
	if limitText != "" {
		limit, e := strconv.ParseInt(limitText, 10, 32)
		if e != nil || limit < 1 {
			return f, fmt.Errorf("usage limit must be a positive whole number")
		}
		f.limit = limit
	}
	if f.start != "" {
		if _, e := parsePromotionDate(f.start, false); e != nil {
			return f, e
		}
	}
	if f.end != "" {
		if _, e := parsePromotionDate(f.end, true); e != nil {
			return f, e
		}
	}
	if f.start != "" && f.end != "" {
		start, _ := parsePromotionDate(f.start, false)
		end, _ := parsePromotionDate(f.end, true)
		if !start.Before(*end) {
			return f, fmt.Errorf("end date must be on or after the start date")
		}
	}
	f.slug = promotionSlug(f.name)
	if f.slug == "" {
		return f, fmt.Errorf("campaign name must include letters or numbers")
	}
	return f, nil
}

func parsePromotionDate(value string, end bool) (*time.Time, error) {
	d, err := time.Parse("2006-01-02", value)
	if err != nil {
		return nil, fmt.Errorf("dates must use YYYY-MM-DD")
	}
	location, err := time.LoadLocation("Asia/Kolkata")
	if err != nil {
		location = time.FixedZone("IST", 19800)
	}
	date := time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, location)
	if end {
		date = date.AddDate(0, 0, 1)
	}
	utc := date.UTC()
	return &utc, nil
}

func promotionSlug(value string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(value) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
			lastDash = false
		} else if !lastDash && b.Len() > 0 {
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

func (h *LivePortalHandler) createPromotion(r *http.Request, actorID int64) error {
	f, err := readPromotionForm(r)
	if err != nil {
		return err
	}
	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		return err
	}
	defer tx.Rollback(r.Context())
	var id int64
	start, end := promotionDateArgs(f)
	if err = tx.QueryRow(r.Context(), `INSERT INTO promotions(slug,name,description,discount_type,discount_value,status,starts_at,ends_at,usage_limit,created_by) VALUES($1,$2,$3,$4,$5,'draft',$6,$7,NULLIF($8,0),$9) RETURNING id`, f.slug, f.name, f.description, f.kind, f.value, start, end, f.limit, actorID).Scan(&id); err != nil {
		return err
	}
	if _, err = tx.Exec(r.Context(), `INSERT INTO coupons(promotion_id,code,usage_limit) VALUES($1,$2,NULLIF($3,0))`, id, f.code, f.limit); err != nil {
		return err
	}
	return tx.Commit(r.Context())
}

func (h *LivePortalHandler) updatePromotion(r *http.Request, id, _ int64) error {
	f, err := readPromotionForm(r)
	if err != nil {
		return err
	}
	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		return err
	}
	defer tx.Rollback(r.Context())
	start, end := promotionDateArgs(f)
	result, err := tx.Exec(r.Context(), `UPDATE promotions SET name=$1,slug=$2,description=$3,discount_type=$4,discount_value=$5,starts_at=$6,ends_at=$7,usage_limit=NULLIF($8,0),updated_at=NOW() WHERE id=$9`, f.name, f.slug, f.description, f.kind, f.value, start, end, f.limit, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("campaign not found")
	}
	result, err = tx.Exec(r.Context(), `UPDATE coupons SET code=$1,usage_limit=NULLIF($2,0),active=TRUE WHERE promotion_id=$3`, f.code, f.limit, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		if _, err = tx.Exec(r.Context(), `INSERT INTO coupons(promotion_id,code,usage_limit) VALUES($1,$2,NULLIF($3,0))`, id, f.code, f.limit); err != nil {
			return err
		}
	}
	return tx.Commit(r.Context())
}

func promotionDateArgs(f promotionForm) (*time.Time, *time.Time) {
	var start, end *time.Time
	if f.start != "" {
		start, _ = parsePromotionDate(f.start, false)
	}
	if f.end != "" {
		end, _ = parsePromotionDate(f.end, true)
	}
	return start, end
}

func (h *LivePortalHandler) deletePromotion(r *http.Request, id int64) error {
	result, err := h.pool.Exec(r.Context(), `DELETE FROM promotions p WHERE p.id=$1 AND NOT EXISTS(SELECT 1 FROM promotion_usages pu WHERE pu.promotion_id=p.id)`, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("campaign not found or already used; archive used campaigns")
	}
	return nil
}

func (h *LivePortalHandler) adminReviewStatus(w http.ResponseWriter, r *http.Request) {
	session, ok := h.session(r)
	if !ok {
		redirectToLogin(w, r)
		return
	}
	root := "/marketplace-admin"
	if r.URL.Path == "/super-admin/reviews/status" {
		root = "/super-admin"
	}
	if !h.hasRole(r, session.UserID, "marketplace_admin", "super_admin") {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid review decision", http.StatusBadRequest)
		return
	}
	token := r.Form.Get("csrf_token")
	if subtle.ConstantTimeCompare([]byte(session.CSRFToken), []byte(token)) != 1 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	reviewID, err := strconv.ParseInt(strings.TrimSpace(r.Form.Get("review_id")), 10, 64)
	decision, reason := strings.TrimSpace(r.Form.Get("decision")), strings.TrimSpace(r.Form.Get("reason"))
	if err != nil || reviewID <= 0 || (decision != "published" && decision != "rejected") || decision == "rejected" && (len([]rune(reason)) < 4 || len([]rune(reason)) > 1000) {
		http.Error(w, "provide a valid review decision and a rejection reason when rejecting", http.StatusUnprocessableEntity)
		return
	}
	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		http.Error(w, "review could not be updated", http.StatusServiceUnavailable)
		return
	}
	defer tx.Rollback(r.Context())
	var productID int64
	err = tx.QueryRow(r.Context(), `UPDATE product_reviews SET status=$1,updated_at=NOW() WHERE id=$2 AND status='pending' RETURNING product_id`, decision, reviewID).Scan(&productID)
	if err != nil {
		http.Error(w, "review is missing or has already been moderated", http.StatusConflict)
		return
	}
	if decision == "published" {
		if _, err = tx.Exec(r.Context(), `
			UPDATE products p SET rating=COALESCE(review_stats.average_rating,0),
				review_count=COALESCE(review_stats.review_count,0), updated_at=NOW()
			FROM (SELECT ROUND(AVG(rating),1)::numeric(2,1) AS average_rating, COUNT(*)::integer AS review_count
			      FROM product_reviews WHERE product_id=$1 AND status='published') review_stats
			WHERE p.id=$1`, productID); err != nil {
			http.Error(w, "product rating could not be updated", http.StatusServiceUnavailable)
			return
		}
	}
	if _, err = tx.Exec(r.Context(), `INSERT INTO audit_logs (actor_id,action,resource_type,resource_id,request_id,metadata) VALUES ($1,$2,'product_review',$3,$4,jsonb_build_object('decision',$2::text,'reason',$5::text))`, strconv.FormatInt(session.UserID, 10), "review."+decision, strconv.FormatInt(reviewID, 10), r.Header.Get("X-Request-ID"), reason); err != nil {
		http.Error(w, "review audit could not be written", http.StatusServiceUnavailable)
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		http.Error(w, "review could not be updated", http.StatusServiceUnavailable)
		return
	}
	http.Redirect(w, r, root+"/reviews?notice=Review+"+url.QueryEscape(decision), http.StatusSeeOther)
}

func (h *LivePortalHandler) adminCampaigns(r *http.Request) []viewmodels.AdminCampaign {
	rows, err := h.pool.Query(r.Context(), `
		SELECT p.id,p.name,p.description,COALESCE(c.code,''),p.discount_type,p.discount_value,p.status,
		       COALESCE(to_char(p.starts_at AT TIME ZONE 'UTC', 'DD Mon YYYY'), 'Not scheduled'),
		       COALESCE(to_char(p.ends_at AT TIME ZONE 'UTC', 'DD Mon YYYY'), 'No end date'),
	       COALESCE(to_char(p.starts_at AT TIME ZONE 'Asia/Kolkata', 'YYYY-MM-DD'),''),
	       COALESCE(to_char(p.ends_at AT TIME ZONE 'Asia/Kolkata' - INTERVAL '1 day', 'YYYY-MM-DD'),''),
	       COUNT(pp.product_id),COALESCE(c.usage_limit,p.usage_limit,0),COALESCE(c.usage_count,0)
		FROM promotions p LEFT JOIN promotion_products pp ON pp.promotion_id = p.id
		LEFT JOIN LATERAL (SELECT code,usage_limit,usage_count FROM coupons WHERE promotion_id=p.id ORDER BY id LIMIT 1) c ON TRUE
		GROUP BY p.id,c.code,c.usage_limit,c.usage_count ORDER BY p.id DESC LIMIT 50`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var result []viewmodels.AdminCampaign
	for rows.Next() {
		var item viewmodels.AdminCampaign
		var value int64
		var productCount int64
		var usageLimit, usageCount int64
		if rows.Scan(&item.ID, &item.Name, &item.Description, &item.Code, &item.Type, &value, &item.Status, &item.Start, &item.End, &item.StartsOn, &item.EndsOn, &productCount, &usageLimit, &usageCount) == nil {
			appliesTo := "All eligible products"
			if productCount > 0 {
				appliesTo = fmt.Sprintf("%d selected products", productCount)
			}
			item.Channel = appliesTo
			item.Value = strconv.FormatInt(value, 10)
			if item.Type == "fixed" {
				item.Value = strconv.FormatFloat(float64(value)/100, 'f', 2, 64)
			}
			item.UsageCount = strconv.FormatInt(usageCount, 10)
			if usageLimit > 0 {
				item.UsageLimit = strconv.FormatInt(usageLimit, 10)
			}
			result = append(result, item)
		}
	}
	return result
}

func (h *LivePortalHandler) adminSettlements(r *http.Request) []viewmodels.AdminSettlement {
	rows, err := h.pool.Query(r.Context(), `
		SELECT ss.id,s.display_name,ss.reference,ss.gross_cents,ss.commission_cents,ss.net_cents,ss.status,
		       ss.period_start,ss.period_end,
	       COUNT(DISTINCT sl.order_id) FILTER (WHERE sl.entry_type='sale')
		FROM seller_settlements ss JOIN sellers s ON s.id=ss.seller_id
		LEFT JOIN seller_ledger sl ON sl.settlement_id=ss.id
		GROUP BY ss.id,s.display_name ORDER BY ss.id DESC LIMIT 50`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var result []viewmodels.AdminSettlement
	for rows.Next() {
		var item viewmodels.AdminSettlement
		var id, gross, commission, amount, orderCount int64
		var periodStart, periodEnd time.Time
		if rows.Scan(&id, &item.Seller, &item.Reference, &gross, &commission, &amount, &item.Status, &periodStart, &periodEnd, &orderCount) == nil {
			item.ID = strconv.FormatInt(id, 10)
			item.Orders = strconv.FormatInt(orderCount, 10)
			item.Gross, item.Commission, item.Amount = money(gross), money(commission), money(amount)
			item.Date = periodEnd.Format("2 Jan 2006")
			item.Period = periodStart.Format("2 Jan 2006") + " – " + periodEnd.Format("2 Jan 2006")
			result = append(result, item)
		}
	}
	return result
}

func isSettlementMutationPath(path string) bool {
	return path == "/finance-portal/settlements/create" || path == "/finance-portal/settlements/status" ||
		path == "/super-admin/finance/settlements/create" || path == "/super-admin/finance/settlements/status"
}

func (h *LivePortalHandler) adminSettlementMutation(w http.ResponseWriter, r *http.Request) {
	root, role := "/finance-portal", "finance_operator"
	if strings.HasPrefix(r.URL.Path, "/super-admin/") {
		root, role = "/super-admin/finance", "super_admin"
	}
	session, ok := h.session(r)
	if !ok {
		redirectToLogin(w, r)
		return
	}
	if r.Method != http.MethodPost || !h.hasRole(r, session.UserID, role) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid settlement request", http.StatusBadRequest)
		return
	}
	if subtle.ConstantTimeCompare([]byte(session.CSRFToken), []byte(r.Form.Get("csrf_token"))) != 1 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	action := strings.TrimPrefix(r.URL.Path, root+"/settlements/")
	var err error
	if action == "create" {
		err = h.createSettlementBatches(r, session.UserID)
	} else if action == "status" {
		id, parseErr := strconv.ParseInt(strings.TrimSpace(r.Form.Get("settlement_id")), 10, 64)
		status := strings.TrimSpace(r.Form.Get("status"))
		if parseErr != nil || id <= 0 || (status != "approved" && status != "paid" && status != "failed") {
			err = fmt.Errorf("invalid settlement decision")
		} else {
			err = h.updateSettlementStatus(r, id, status, session.UserID)
		}
	} else {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "settlement could not be updated: "+err.Error(), http.StatusConflict)
		return
	}
	_, _ = h.pool.Exec(r.Context(), `INSERT INTO audit_logs(actor_id,action,resource_type,resource_id,request_id,metadata) VALUES($1,$2,'seller_settlement',$3,$4,jsonb_build_object('status',$5::text))`, strconv.FormatInt(session.UserID, 10), "settlement."+action, r.Form.Get("settlement_id"), r.Header.Get("X-Request-ID"), r.Form.Get("status"))
	destination := root + "?workspace=settlements&notice=Settlement+workflow+saved"
	if root == "/finance-portal" {
		destination = root + "/settlements?notice=Settlement+workflow+saved"
	}
	http.Redirect(w, r, destination, http.StatusSeeOther)
}

type settlementBatch struct {
	sellerID, gross, commission, net int64
	start, end                       time.Time
}

func (h *LivePortalHandler) createSettlementBatches(r *http.Request, actorID int64) error {
	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		return err
	}
	defer tx.Rollback(r.Context())
	if _, err = tx.Exec(r.Context(), `SELECT pg_advisory_xact_lock(4856203)`); err != nil {
		return err
	}
	rows, err := tx.Query(r.Context(), `SELECT seller_id,SUM(gross_cents)::BIGINT,SUM(commission_cents)::BIGINT,SUM(net_cents)::BIGINT,MIN(created_at)::date,MAX(created_at)::date FROM seller_ledger WHERE status='open' AND settlement_id IS NULL GROUP BY seller_id HAVING SUM(net_cents)>0 ORDER BY seller_id`)
	if err != nil {
		return err
	}
	batches := make([]settlementBatch, 0)
	for rows.Next() {
		var item settlementBatch
		if err := rows.Scan(&item.sellerID, &item.gross, &item.commission, &item.net, &item.start, &item.end); err != nil {
			rows.Close()
			return err
		}
		batches = append(batches, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	for _, item := range batches {
		reference := fmt.Sprintf("SETTLE-%d-%s", item.sellerID, time.Now().UTC().Format("20060102T150405.000000000Z"))
		var settlementID int64
		if err := tx.QueryRow(r.Context(), `INSERT INTO seller_settlements(seller_id,period_start,period_end,gross_cents,commission_cents,net_cents,status,reference) VALUES($1,$2,$3,$4,$5,$6,'pending',$7) RETURNING id`, item.sellerID, item.start, item.end, item.gross, item.commission, item.net, reference).Scan(&settlementID); err != nil {
			return err
		}
		if _, err := tx.Exec(r.Context(), `UPDATE seller_ledger SET settlement_id=$1 WHERE seller_id=$2 AND status='open' AND settlement_id IS NULL`, settlementID, item.sellerID); err != nil {
			return err
		}
	}
	if _, err = tx.Exec(r.Context(), `INSERT INTO audit_logs(actor_id,action,resource_type,resource_id,request_id,metadata) VALUES($1,'settlement.batch_created','seller_settlement','batch',$2,jsonb_build_object('batch_count',$3::integer))`, strconv.FormatInt(actorID, 10), r.Header.Get("X-Request-ID"), len(batches)); err != nil {
		return err
	}
	return tx.Commit(r.Context())
}

func (h *LivePortalHandler) updateSettlementStatus(r *http.Request, id int64, status string, actorID int64) error {
	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		return err
	}
	defer tx.Rollback(r.Context())
	var current string
	if err := tx.QueryRow(r.Context(), `SELECT status FROM seller_settlements WHERE id=$1 FOR UPDATE`, id).Scan(&current); err != nil {
		return err
	}
	if current == "pending" && status != "approved" || current == "approved" && status != "paid" && status != "failed" {
		return fmt.Errorf("invalid state transition from %s to %s", current, status)
	}
	if _, err := tx.Exec(r.Context(), `UPDATE seller_settlements SET status=$1,updated_at=NOW() WHERE id=$2`, status, id); err != nil {
		return err
	}
	switch status {
	case "paid":
		if _, err := tx.Exec(r.Context(), `UPDATE seller_ledger SET status='settled' WHERE settlement_id=$1 AND status='open'`, id); err != nil {
			return err
		}
	case "failed":
		if _, err := tx.Exec(r.Context(), `UPDATE seller_ledger SET settlement_id=NULL WHERE settlement_id=$1 AND status='open'`, id); err != nil {
			return err
		}
	}
	_, err = tx.Exec(r.Context(), `INSERT INTO audit_logs(actor_id,action,resource_type,resource_id,request_id,metadata) VALUES($1,$2,'seller_settlement',$3,$4,jsonb_build_object('previous_status',$5::text,'status',$6::text))`, strconv.FormatInt(actorID, 10), "settlement."+status, strconv.FormatInt(id, 10), r.Header.Get("X-Request-ID"), current, status)
	if err != nil {
		return err
	}
	return tx.Commit(r.Context())
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
	rows, err := h.pool.Query(r.Context(), `SELECT sl.id,sl.created_at,sl.gross_cents,sl.commission_cents,sl.net_cents,sl.status,sl.entry_type,COALESCE(o.order_number,'') FROM seller_ledger sl LEFT JOIN orders o ON o.id=sl.order_id WHERE sl.seller_id=$1 ORDER BY sl.id DESC LIMIT 50`, sellerID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var result []viewmodels.SellerAdminLedger
	for rows.Next() {
		var id int64
		var created time.Time
		var gross, commission, net int64
		var status, kind, orderNumber string
		if rows.Scan(&id, &created, &gross, &commission, &net, &status, &kind, &orderNumber) == nil {
			entry := viewmodels.SellerAdminLedger{Number: orderNumber, Date: created.Format("2 Jan 2006"), Net: money(net), Status: status}
			if entry.Number == "" {
				entry.Number = fmt.Sprintf("LEDGER-%d", id)
			}
			if kind == "refund" {
				entry.Sale = "—"
				entry.Commission = "—"
				entry.Adjustments = money(net)
			} else {
				entry.Sale = money(gross)
				entry.Commission = money(commission)
				entry.Adjustments = "INR 0.00"
			}
			result = append(result, entry)
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
	bodyHTML := strings.ReplaceAll(body.String(), `href="/finance-portal`, `href="__CANONICAL_FINANCE_LINK__`)
	bodyHTML = strings.ReplaceAll(bodyHTML, `action="/finance-portal`, `action="__CANONICAL_FINANCE_ACTION__`)
	bodyHTML = replacer.Replace(bodyHTML)
	bodyHTML = strings.ReplaceAll(bodyHTML, `href="__CANONICAL_FINANCE_LINK__`, `href="/finance-portal`)
	bodyHTML = strings.ReplaceAll(bodyHTML, `action="__CANONICAL_FINANCE_ACTION__`, `action="/finance-portal`)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(bodyHTML))
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
