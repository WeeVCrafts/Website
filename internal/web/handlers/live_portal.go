package handlers

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	applicationauth "github.com/wecratfs/commerce/internal/application/auth"
	applicationcommerce "github.com/wecratfs/commerce/internal/application/commerce"
	domaincommerce "github.com/wecratfs/commerce/internal/domain/commerce"
	domainidentity "github.com/wecratfs/commerce/internal/domain/identity"
	"github.com/wecratfs/commerce/internal/identity"
	"github.com/wecratfs/commerce/internal/ports"
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
	storefrontPreview  *LiveStorefrontHandler
	categoryStorage    ports.ObjectStorage
	localMode          bool
	pool               *pgxpool.Pool
	sessions           *applicationauth.SessionService
	roleAdmin          *applicationauth.RoleAdminService
	supportAttachments *applicationcommerce.SupportAttachmentService
	fallback           http.Handler
	assets             http.Handler
	paymentMode        string
	runtimeProbes      map[string]func(context.Context) error
}

func (h *LivePortalHandler) EnableLocalDeliverySimulation(enabled bool) { h.localMode = enabled }

// SetRuntimeProbes connects the runtime status page to the configured adapters.
func (h *LivePortalHandler) SetRuntimeProbes(paymentMode string, probes map[string]func(context.Context) error) {
	h.paymentMode, h.runtimeProbes = paymentMode, probes
}

type projectionContextKey struct{}
type projectionState struct {
	failed bool
	pages  map[string]viewmodels.PortalPagination
}

func projectionRequest(r *http.Request) *http.Request {
	if _, ok := r.Context().Value(projectionContextKey{}).(*projectionState); ok {
		return r
	}
	return r.WithContext(context.WithValue(r.Context(), projectionContextKey{}, &projectionState{pages: make(map[string]viewmodels.PortalPagination)}))
}

func recordProjectionError(r *http.Request, err error) {
	if err == nil || errors.Is(err, pgx.ErrNoRows) {
		return
	}
	if state, ok := r.Context().Value(projectionContextKey{}).(*projectionState); ok {
		state.failed = true
	}
}

type projectionRow struct {
	pgx.Row
	request *http.Request
}

func (row projectionRow) Scan(dest ...any) error {
	err := row.Row.Scan(dest...)
	recordProjectionError(row.request, err)
	return err
}

type projectionResult struct {
	pgx.Rows
	request *http.Request
}

func (rows projectionResult) Scan(dest ...any) error {
	err := rows.Rows.Scan(dest...)
	recordProjectionError(rows.request, err)
	return err
}
func (rows projectionResult) Next() bool {
	next := rows.Rows.Next()
	if !next {
		recordProjectionError(rows.request, rows.Rows.Err())
	}
	return next
}
func (h *LivePortalHandler) query(r *http.Request, sql string, args ...any) (pgx.Rows, error) {
	if h.pool == nil {
		err := errors.New("PostgreSQL unavailable")
		recordProjectionError(r, err)
		return nil, err
	}
	rows, err := h.pool.Query(r.Context(), sql, args...)
	recordProjectionError(r, err)
	if err != nil {
		return nil, err
	}
	return projectionResult{Rows: rows, request: r}, nil
}
func (h *LivePortalHandler) queryRow(r *http.Request, sql string, args ...any) pgx.Row {
	if h.pool == nil {
		return failedProjectionRow{request: r}
	}
	return projectionRow{Row: h.pool.QueryRow(r.Context(), sql, args...), request: r}
}

type failedProjectionRow struct{ request *http.Request }

func (row failedProjectionRow) Scan(...any) error {
	err := errors.New("PostgreSQL unavailable")
	recordProjectionError(row.request, err)
	return err
}

func (h *LivePortalHandler) projectionRows(r *http.Request, key, sql string, args ...any) (pgx.Rows, error) {
	pageParameter := "page"
	if key == "seller-ledger" {
		pageParameter = "ledger_page"
	}
	if key == "seller-settlements" {
		pageParameter = "settlement_page"
	}
	page, err := strconv.Atoi(r.URL.Query().Get(pageParameter))
	if err != nil || page < 1 {
		page = 1
	}
	if page > 1000000 {
		page = 1000000
	}
	const size = 50
	var total int64
	if err := h.queryRow(r, "SELECT COUNT(*) FROM ("+sql+") projection_count", args...).Scan(&total); err != nil {
		return nil, err
	}
	pages := int((total + size - 1) / size)
	if pages < 1 {
		pages = 1
	}
	if page > pages {
		page = pages
	}
	pagination := viewmodels.PortalPagination{Page: page, PageSize: size, Total: total, Pages: pages}
	link := func(target int) string {
		u := *r.URL
		q := u.Query()
		q.Set(pageParameter, strconv.Itoa(target))
		u.RawQuery = q.Encode()
		return u.String()
	}
	if page > 1 {
		pagination.PreviousURL = link(page - 1)
	}
	if page < pages {
		pagination.NextURL = link(page + 1)
	}
	if state, ok := r.Context().Value(projectionContextKey{}).(*projectionState); ok {
		state.pages[key] = pagination
	}
	args = append(args, size, (page-1)*size)
	return h.query(r, sql+fmt.Sprintf(" LIMIT $%d OFFSET $%d", len(args)-1, len(args)), args...)
}
func projectionPagination(r *http.Request, key string) viewmodels.PortalPagination {
	if state, ok := r.Context().Value(projectionContextKey{}).(*projectionState); ok {
		return state.pages[key]
	}
	return viewmodels.PortalPagination{}
}
func projectionWarning(r *http.Request, current string) string {
	if state, ok := r.Context().Value(projectionContextKey{}).(*projectionState); ok && state.failed {
		return joinPortalWarning(current, "Live data could not be loaded from PostgreSQL. Refresh the page to try again.")
	}
	return current
}

// denyPortalAccess distinguishes unavailable authorization data from a real
// permission denial while keeping the request closed to unauthorized access.
func denyPortalAccess(w http.ResponseWriter, r *http.Request) {
	if projectionWarning(r, "") != "" {
		http.Error(w, "Live data could not be loaded from PostgreSQL. Refresh the page to try again.", http.StatusServiceUnavailable)
		return
	}
	http.Error(w, "forbidden", http.StatusForbidden)
}
func adminProjectionKey(active, workspace string) string {
	if workspace == "settlements" {
		return "admin-settlements"
	}
	if workspace == "reviews" {
		return "admin-reviews"
	}
	switch active {
	case "products":
		return "admin-products"
	case "inventory":
		return "admin-inventory"
	case "sellers":
		return "admin-sellers"
	case "marketing":
		return "admin-campaigns"
	case "security":
		return "admin-audit"
	case "orders":
		return "admin-orders"
	case "returns":
		return "admin-returns"
	case "finance":
		return "admin-transactions"
	case "support":
		return "admin-cases"
	}
	return ""
}
func sellerProjectionKey(active, tab string) string {
	switch active {
	case "products":
		return "seller-products"
	case "inventory":
		return "seller-inventory"
	case "orders":
		return "seller-orders"
	case "returns":
		return "seller-returns"
	case "earnings":
		if tab == "settlements" || tab == "payouts" {
			return "seller-settlements"
		}
		return "seller-ledger"
	}
	return ""
}
func transactionProjectionStatus(r *http.Request) string {
	if strings.Contains(r.URL.Path, "failed-payments") || r.URL.Query().Get("workspace") == "failed-payments" {
		return "failed"
	}
	if strings.Contains(r.URL.Path, "refunds") || r.URL.Query().Get("workspace") == "refunds" {
		return "refunded"
	}
	return ""
}
func sellerOrderProjectionStatus(r *http.Request) string {
	switch r.URL.Query().Get("tab") {
	case "new":
		return "pending"
	case "ready":
		return "out_for_delivery"
	case "rto":
		return "rto"
	case "pending", "processing", "packing", "shipped", "out_for_delivery", "delivered":
		return r.URL.Query().Get("tab")
	}
	return ""
}
func (h *LivePortalHandler) runtimeConfigs(r *http.Request) []viewmodels.AdminConfig {
	mode := h.paymentMode
	if mode == "" {
		mode = "Not configured"
	}
	result := []viewmodels.AdminConfig{{Icon: "payments", Name: "Payment mode", Value: mode, Detail: "Configured payment adapter"}}
	for _, name := range []string{"database", "redis", "search", "storage"} {
		item := viewmodels.AdminConfig{Icon: name, Name: strings.ToUpper(name[:1]) + name[1:], Value: "Not inspected", Detail: "No readiness probe configured"}
		if probe := h.runtimeProbes[name]; probe != nil {
			ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
			err := probe(ctx)
			cancel()
			item.Value = "Ready"
			item.Detail = "Adapter readiness probe succeeded"
			if err != nil {
				item.Value = "Unavailable"
				item.Detail = "Adapter readiness probe failed"
				if name == "search" {
					item.Detail = "Search unavailable; PostgreSQL fallback is used"
				}
			}
		}
		result = append(result, item)
	}
	return result
}

func (h *LivePortalHandler) SetRoleAdminService(service *applicationauth.RoleAdminService) {
	h.roleAdmin = service
}

func (h *LivePortalHandler) SetSupportAttachmentService(service *applicationcommerce.SupportAttachmentService) {
	h.supportAttachments = service
}

func NewLivePortalHandler(pool *pgxpool.Pool, sessions *applicationauth.SessionService, fallback http.Handler) *LivePortalHandler {
	return &LivePortalHandler{
		pool: pool, sessions: sessions, fallback: fallback,
		assets: http.StripPrefix("/assets/", http.FileServer(http.FS(assets.FS))),
	}
}

func (h *LivePortalHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	r = projectionRequest(r)
	if r.URL.Path == "/ui/admin/orders/status" && r.Method == http.MethodGet {
		h.adminOrderStatusRefresh(w, r)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/assets/") {
		if serveUploadedAsset(w, r) {
			return
		}
		h.assets.ServeHTTP(w, r)
		return
	}
	if strings.Contains(r.URL.Path, "/team") {
		http.NotFound(w, r)
		return
	}
	if h.serveRetiredPortal(w, r) {
		return
	}
	if h.serveNotifications(w, r) {
		return
	}
	if h.serveStorefrontEditor(w, r) || h.serveCompanySupport(w, r) || h.serveManagement(w, r) || h.serveSellerSupport(w, r) {
		return
	}
	session, ok := h.session(r)
	if !ok {
		redirectToLogin(w, r)
		return
	}
	roles := h.userRoles(r, session.UserID)
	if applicationauth.LoginDestination(roles) == "/access-unavailable" {
		denyPortalAccess(w, r)
		return
	}
	currentRole := roles[0]
	if r.URL.Path == "/workspaces" {
		http.Redirect(w, r, applicationauth.LoginDestination(roles), http.StatusSeeOther)
		return
	}
	if pathUnder(r.URL.Path, "/seller") {
		if r.Method == http.MethodGet {
			target := legacySellerPortalDestination(r.URL.Path)
			if target == "" || currentRole != domainidentity.RoleSeller {
				denyPortalAccess(w, r)
				return
			}
			redirectPortalDestination(w, r, target)
			return
		}
		if currentRole != domainidentity.RoleSeller {
			denyPortalAccess(w, r)
			return
		}
		h.delegateWithPortalRedirect(w, r, "/seller-admin")
		return
	}
	if r.Method != http.MethodGet {
		if _, root, canonical := canonicalAdminPortal(r.URL.Path); canonical {
			if !roleAllowsPortalPath(currentRole, r.URL.Path) {
				denyPortalAccess(w, r)
				return
			}
			if r.URL.Path == "/super-admin/inventory/adjust" {
				h.delegateWithPortalRedirect(w, r, root)
				return
			}
			if isSettlementMutationPath(r.URL.Path) {
				h.adminSettlementMutation(w, r)
				return
			}
			if isPromotionMutationPath(r.URL.Path) {
				h.adminPromotionMutation(w, r)
				return
			}
			if strings.HasSuffix(r.URL.Path, "/reviews/status") {
				h.adminReviewStatus(w, r)
				return
			}
			h.delegateWithPortalRedirect(w, requestWithPath(r, adminLegacyPagePath(r.URL.Path, root)), root)
			return
		}
		if pathUnder(r.URL.Path, "/seller-admin") {
			if currentRole != domainidentity.RoleSeller {
				denyPortalAccess(w, r)
				return
			}
			if r.URL.Path == "/seller-admin/inventory/threshold" {
				h.updateSellerLowStockThreshold(w, r)
				return
			}
			if legacy := canonicalSellerMutationPath(r.URL.Path); legacy != "" {
				if r.URL.Path == "/seller-admin/products" || isSellerProductEditPath(strings.TrimPrefix(r.URL.Path, "/seller-admin/")) {
					h.delegateSellerProduct(w, r, legacy, session.UserID, session.CSRFToken)
					return
				}
				h.delegateWithPortalRedirect(w, requestWithPath(r, legacy), "/seller-admin")
				return
			}
			http.NotFound(w, r)
			return
		}
		h.delegate(w, r)
		return
	}
	if r.URL.Path == "/finance" || r.URL.Path == "/finance/" {
		target := legacyAdminDestination("/finance", roles)
		if target == "" {
			denyPortalAccess(w, r)
			return
		}
		redirectPortalDestination(w, r, target)
		return
	}
	switch {
	case isCanonicalAdminPortal(r.URL.Path):
		_, root, _ := canonicalAdminPortal(r.URL.Path)
		if !roleAllowsPortalPath(currentRole, r.URL.Path) {
			denyPortalAccess(w, r)
			return
		}
		if isSuperAdminProductEditorPath(r.URL.Path) {
			h.renderSuperAdminProductEditor(w, r, session.UserID, h.userDisplayName(r, session.UserID), session.CSRFToken)
			return
		}
		if !knownAdminPage(strings.TrimPrefix(r.URL.Path, root)) {
			http.NotFound(w, r)
			return
		}
		h.renderAdmin(w, requestWithPath(r, adminLegacyPagePath(r.URL.Path, root)), session.UserID, currentRole, root, h.userDisplayName(r, session.UserID), session.CSRFToken)
	case pathUnder(r.URL.Path, "/seller-admin"):
		if currentRole != domainidentity.RoleSeller || !sellerPortalAllowed(r.URL.Path, true, nil) {
			denyPortalAccess(w, r)
			return
		}
		_, assigned := h.sellerPermissions(r, session.UserID)
		if !assigned {
			denyPortalAccess(w, r)
			return
		}
		name := h.userDisplayName(r, session.UserID)
		if r.URL.Path == "/seller-admin/products/new" || isSellerProductFormPath(r.URL.Path) {
			h.renderSellerProductForm(w, r, session.UserID, name, true, nil, session.CSRFToken)
			return
		}
		if r.URL.Path == "/seller-admin/terms" {
			h.renderSellerTerms(w, r, session.UserID, name, true, nil, session.CSRFToken)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/seller-admin/orders/") && sellerOrderDetailPath(strings.TrimPrefix(r.URL.Path, "/seller-admin/")) {
			h.renderSellerOrderDetail(w, r, session.UserID, name, true, nil, session.CSRFToken)
			return
		}
		h.renderSeller(w, r, session.UserID, name, true, nil, session.CSRFToken)
	default:
		h.delegate(w, r)
	}
}

func (h *LivePortalHandler) adminOrderStatusRefresh(w http.ResponseWriter, r *http.Request) {
	session, ok := h.session(r)
	if !ok {
		http.Error(w, "Sign in to refresh order status", http.StatusUnauthorized)
		return
	}
	roles := h.userRoles(r, session.UserID)
	allowed := false
	for _, role := range roles {
		if role == domainidentity.RoleAdmin || role == domainidentity.RoleSuperAdmin {
			allowed = true
			break
		}
	}
	if !allowed {
		http.Error(w, "Order status access denied", http.StatusForbidden)
		return
	}
	orders := h.adminOrders(r)
	if projectionWarning(r, "") != "" {
		http.Error(w, "Order updates are temporarily unavailable", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = adminpages.OrderQueueRegion(orders).Render(r.Context(), w)
}

// Keep old bookmarks without retaining separate operational identities.
func (h *LivePortalHandler) serveRetiredPortal(w http.ResponseWriter, r *http.Request) bool {
	for _, root := range []string{"/marketplace-admin", "/finance-portal", "/operations-portal", "/support-portal", "/support"} {
		if !pathUnder(r.URL.Path, root) {
			continue
		}
		session, ok := h.session(r)
		if !ok {
			redirectToLogin(w, r)
			return true
		}
		roles := h.userRoles(r, session.UserID)
		if len(roles) != 1 || !domainidentity.CompanyRole(roles[0]) {
			denyPortalAccess(w, r)
			return true
		}
		dest := applicationauth.LoginDestination(roles)
		relative := strings.TrimPrefix(r.URL.Path, root)
		if root == "/support-portal" || root == "/support" {
			relative = "/support" + relative
		}
		if root == "/finance-portal" && relative == "" {
			relative = "/finance"
		}
		if root == "/operations-portal" && relative == "" {
			relative = "/returns"
		}
		target := dest + relative
		if !roleAllowsPortalPath(roles[0], target) {
			denyPortalAccess(w, r)
			return true
		}
		if r.Method == http.MethodGet {
			redirectPortalDestination(w, r, target)
			return true
		}
		// Rewrite internally: never redirect a POST to a different operation.
		h.ServeHTTP(w, requestWithPath(r, target))
		return true
	}
	return false
}

// serveUploadedAsset serves product images saved by the legacy admin form.
// The embedded asset filesystem cannot see files written after compilation,
// so this path is handled explicitly and restricted to a single directory.
func serveUploadedAsset(w http.ResponseWriter, r *http.Request) bool {
	const prefix = "/assets/images/uploads/"
	if !strings.HasPrefix(r.URL.Path, prefix) {
		return false
	}
	name := strings.TrimPrefix(r.URL.Path, prefix)
	if name == "" || name != filepath.Base(name) || strings.ContainsAny(name, `/\\`) {
		http.NotFound(w, r)
		return true
	}
	path := filepath.Join("web", "assets", "images", "uploads", name)
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		http.NotFound(w, r)
		return true
	}
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	http.ServeFile(w, r, path)
	return true
}

func (h *LivePortalHandler) renderSellerTerms(w http.ResponseWriter, r *http.Request, userID int64, displayName string, isOwner bool, permissions []string, csrfToken string) {
	page := h.sellerPage(requestWithPath(r, "/seller-admin"), userID, isOwner, permissions)
	page.Active = "terms"
	page.Title = "Seller Terms & Conditions"
	page.Subtitle = "Review the marketplace requirements for your seller account."
	page.DisplayName = displayName
	page.UserEmail = h.userEmail(r, userID)
	page.DataWarning = projectionWarning(r, page.DataWarning)
	page.Initials = initials(displayName)
	page.CSRFToken = csrfToken
	page.RoleLabel = "Seller"
	h.render(w, r, sellerpages.PortalTerms(page))
}

func (h *LivePortalHandler) renderSellerOrderDetail(w http.ResponseWriter, r *http.Request, userID int64, displayName string, isOwner bool, permissions []string, csrfToken string) {
	number := strings.TrimPrefix(r.URL.Path, "/seller-admin/orders/")
	page := h.sellerPage(requestWithPath(r, "/seller-admin/orders"), userID, isOwner, permissions)
	page.Active = "orders"
	page.Title = "Order details"
	page.Subtitle = "Seller-scoped items, delivery details and fulfilment tracking."
	page.DisplayName = displayName
	page.Initials = initials(displayName)
	page.CSRFToken = csrfToken
	page.CanProducts = sellerPortalAllowed("/seller-admin/products", isOwner, permissions)
	page.CanProductWrite = page.SellerStatus == "active" && (isOwner)
	page.CanInventory = sellerPortalAllowed("/seller-admin/inventory", isOwner, permissions)
	page.CanInventoryWrite = isOwner && page.SellerStatus == "active"
	page.CanOrders = true
	page.CanReturns = sellerPortalAllowed("/seller-admin/returns", isOwner, permissions)
	page.UserEmail = h.userEmail(r, userID)
	page.CanFulfill = isOwner && page.SellerStatus == "active"
	page.CanReports = sellerPortalAllowed("/seller-admin/analytics", isOwner, permissions)
	page.CanManageStore = isOwner
	page.Notice = r.URL.Query().Get("notice")
	detail := &viewmodels.SellerAdminOrderDetail{}
	var created time.Time
	var subtotal int64
	err := h.queryRow(r, `SELECT s.id::text,o.order_number,o.created_at,u.display_name,o.status,COALESCE(sf.status,'pending'),COALESCE(pa.status,'created'),COALESCE(sf.carrier,''),COALESCE(sf.tracking_number,''),COALESCE(sf.last_note,''),COALESCE(sf.revision,0)::text,COALESCE(o.address_snapshot->>'recipient_name',''),COALESCE(o.address_snapshot->>'line1',''),COALESCE(o.address_snapshot->>'line2',''),COALESCE(o.address_snapshot->>'city',''),COALESCE(o.address_snapshot->>'state',''),COALESCE(o.address_snapshot->>'postal_code',''),COALESCE(o.address_snapshot->>'country_code','IN'),COALESCE(SUM(oi.line_total_cents),0),o.currency FROM orders o JOIN users u ON u.id=o.user_id JOIN order_items oi ON oi.order_id=o.id JOIN product_variants v ON v.id=oi.variant_id JOIN products p ON p.id=v.product_id JOIN sellers s ON s.id=p.seller_id LEFT JOIN seller_fulfillments sf ON sf.order_id=o.id AND sf.seller_id=s.id LEFT JOIN LATERAL (SELECT status FROM payment_attempts WHERE order_id=o.id ORDER BY id DESC LIMIT 1) pa ON TRUE WHERE o.order_number=$1 AND s.id=(SELECT ss.id FROM sellers ss  WHERE ss.owner_user_id=$2 ORDER BY CASE WHEN ss.owner_user_id=$2 THEN 0 ELSE 1 END,ss.id LIMIT 1) AND (s.owner_user_id=$2 ) GROUP BY s.id,o.id,u.display_name,sf.status,sf.carrier,sf.tracking_number,sf.last_note,sf.revision,pa.status`, number, userID).Scan(&detail.SellerID, &detail.Number, &created, &detail.Customer, &detail.OrderStatus, &detail.Status, &detail.Payment, &detail.Carrier, &detail.TrackingNumber, &detail.Note, &detail.Revision, &detail.RecipientName, &detail.Line1, &detail.Line2, &detail.City, &detail.State, &detail.PostalCode, &detail.CountryCode, &subtotal, &detail.Currency)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.NotFound(w, r)
		} else {
			http.Error(w, "order details unavailable", http.StatusServiceUnavailable)
		}
		return
	}
	detail.Date = created.Format("2 Jan 2006")
	detail.Subtotal = money(subtotal)
	rows, err := h.query(r, `SELECT oi.product_name,oi.sku,oi.quantity,oi.unit_price_cents,oi.line_total_cents FROM orders o JOIN order_items oi ON oi.order_id=o.id JOIN product_variants v ON v.id=oi.variant_id JOIN products p ON p.id=v.product_id JOIN sellers s ON s.id=p.seller_id WHERE o.order_number=$1 AND s.id=$2 AND (s.owner_user_id=$3 ) ORDER BY oi.id`, number, detail.SellerID, userID)
	if err != nil {
		http.Error(w, "order details unavailable", http.StatusServiceUnavailable)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var item viewmodels.SellerAdminOrderItem
		var qty int64
		var unit, total int64
		if rows.Scan(&item.Name, &item.SKU, &qty, &unit, &total) == nil {
			item.Quantity = strconv.FormatInt(qty, 10)
			item.UnitPrice = money(unit)
			item.LineTotal = money(total)
			detail.Items = append(detail.Items, item)
		}
	}
	page.OrderDetail = detail
	page.OrderReturnPage = page.Pagination.Page
	page.Pagination = viewmodels.PortalPagination{}
	h.render(w, r, sellerpages.PortalOrderDetail(page))
}

func (h *LivePortalHandler) updateSellerLowStockThreshold(w http.ResponseWriter, r *http.Request) {
	session, ok := h.session(r)
	if !ok {
		redirectToLogin(w, r)
		return
	}
	if token := r.FormValue("csrf_token"); token == "" || subtle.ConstantTimeCompare([]byte(token), []byte(session.CSRFToken)) != 1 {
		denyPortalAccess(w, r)
		return
	}
	roles := h.userRoles(r, session.UserID)
	owner := hasIdentityRole(roles, domainidentity.RoleSeller)
	_, assigned := h.sellerPermissions(r, session.UserID)
	if !assigned || !owner {
		denyPortalAccess(w, r)
		return
	}
	threshold, err := strconv.Atoi(strings.TrimSpace(r.FormValue("low_stock_threshold")))
	if err != nil || threshold < 1 || threshold > 100000 {
		http.Error(w, "threshold must be between 1 and 100000", http.StatusUnprocessableEntity)
		return
	}
	result, err := h.pool.Exec(r.Context(), `UPDATE sellers SET low_stock_threshold=$1,updated_at=NOW() WHERE id=(SELECT s.id FROM sellers s  WHERE s.owner_user_id=$2 ORDER BY CASE WHEN s.owner_user_id=$2 THEN 0 ELSE 1 END,s.id LIMIT 1)`, threshold, session.UserID)
	if err != nil || result.RowsAffected() == 0 {
		http.Error(w, "could not save threshold", http.StatusServiceUnavailable)
		return
	}
	http.Redirect(w, r, "/seller-admin/inventory?notice=Low+stock+threshold+saved", http.StatusSeeOther)
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
	if userID <= 0 || len(roles) == 0 {
		return false
	}
	var allowed bool
	if err := h.queryRow(r, `SELECT EXISTS (SELECT 1 FROM user_roles WHERE user_id=$1 AND role_slug = ANY($2::text[]))`, userID, roles).Scan(&allowed); err != nil {
		return false
	}
	return allowed
}

func (h *LivePortalHandler) userRoles(r *http.Request, userID int64) []domainidentity.Role {
	if userID <= 0 {
		return nil
	}
	rows, err := h.query(r, `SELECT role_slug FROM user_roles WHERE user_id=$1 ORDER BY role_slug`, userID)
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
		return "WeevCrafts account"
	}
	var name string
	if err := h.queryRow(r, `SELECT display_name FROM users WHERE id=$1`, userID).Scan(&name); err != nil || strings.TrimSpace(name) == "" {
		return "WeevCrafts account"
	}
	return identity.DisplayName(name)
}

func (h *LivePortalHandler) userEmail(r *http.Request, userID int64) string {
	if h.pool == nil || userID <= 0 {
		return ""
	}
	var email string
	_ = h.queryRow(r, `SELECT email FROM users WHERE id=$1`, userID).Scan(&email)
	return email
}

func (h *LivePortalHandler) sellerPermissions(r *http.Request, userID int64) ([]string, bool) {
	if userID <= 0 {
		return nil, false
	}
	var permissions []string
	var assigned bool
	err := h.queryRow(r, `
		SELECT ARRAY[]::text[], (s.owner_user_id=$1)
		FROM sellers s

		WHERE s.owner_user_id=$1
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
	r = projectionRequest(r)
	page := h.adminPage(r, actorID, role)
	page.PortalRoot = root
	page.SettlementRoot = root
	page.SettlementRoot = root + "/finance"
	page.PortalLabel = portalLabel(role)
	page.RoleLabel = roleLabel(role)
	page.DisplayName = displayName
	page.CSRFToken = csrfToken
	page.CanFinance = role == domainidentity.RoleSuperAdmin || role == domainidentity.RoleAdmin
	page.CanSecurity = role == domainidentity.RoleSuperAdmin
	page.CanManageCatalog = domainidentity.CompanyRole(role)
	page.IsFinance = false
	page.IsOperations = false
	page.NavActive = page.Active
	if page.Workspace == "roles" {
		page.NavActive = "roles"
	}
	if page.Active == "security" && page.Workspace == "users" {
		page.NavActive = "users"
	}
	if page.IsFinance && page.Workspace != "" {
		page.NavActive = page.Workspace
	}
	page.Stats = h.portalStats(r, role)
	page.DataWarning = projectionWarning(r, page.DataWarning)
	if page.DataWarning != "" {
		page.Stats = nil
	}
	if domainidentity.CompanyRole(role) {
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
		page.Pagination = projectionPagination(r, "admin-inventory")
		if page.Inventory == nil {
			page.DataWarning = joinPortalWarning(page.DataWarning, "The inventory list could not be loaded. Refresh the page to try again.")
			page.Stats = nil
		}
		component = adminpages.PortalInventory(page)
	case "orders":
		component = adminpages.PortalOrders(page)
	case "returns":
		component = adminpages.PortalReturns(page)
	case "finance":
		component = adminpages.PortalFinance(page)
	case "support":
		if page.Workspace == "reviews" {
			component = adminpages.PortalReviews(page)
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
		http.NotFound(w, r)
		return
	}
	h.renderAdminComponent(w, r, component, page)
}

func isSuperAdminProductEditorPath(path string) bool {
	if path == "/super-admin/products/new" || path == "/admin/products/new" {
		return true
	}
	if !(strings.HasPrefix(path, "/super-admin/products/") || strings.HasPrefix(path, "/admin/products/")) || !strings.HasSuffix(path, "/edit") {
		return false
	}
	id := strings.TrimSuffix(strings.TrimPrefix(strings.TrimPrefix(path, "/super-admin/products/"), "/admin/products/"), "/edit")
	parsed, err := strconv.ParseInt(id, 10, 64)
	return err == nil && parsed > 0 && !strings.Contains(id, "/")
}

func (h *LivePortalHandler) renderSuperAdminProductEditor(w http.ResponseWriter, r *http.Request, actorID int64, displayName, csrfToken string) {
	root := "/admin"
	role := domainidentity.RoleAdmin
	if strings.HasPrefix(r.URL.Path, "/super-admin/") {
		root = "/super-admin"
		role = domainidentity.RoleSuperAdmin
	}
	pageRequest := requestWithPath(r, "/admin/products")
	page := h.adminPage(pageRequest, actorID, role)
	page.PortalRoot = root
	page.PortalLabel = portalLabel(role)
	page.RoleLabel = roleLabel(role)
	page.DisplayName = displayName
	page.CSRFToken = csrfToken
	page.CanFinance, page.CanSecurity, page.CanManageCatalog = true, role == domainidentity.RoleSuperAdmin, true
	page.Stats = h.portalStats(r, role)
	page.Sellers = h.adminSellers(r)

	form := viewmodels.AdminProductEditor{Action: r.URL.Path, InitialStock: "0", DeliveryLabel: "Delivery to serviceable PIN codes in India"}
	if r.URL.Path == root+"/products/new" {
		page.Title = "Add product"
		page.Subtitle = "Create a product draft for an active Indian seller, then submit it for marketplace review."
	} else {
		idText := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, root+"/products/"), "/edit")
		productID, err := strconv.ParseInt(idText, 10, 64)
		if err != nil || productID <= 0 {
			http.NotFound(w, r)
			return
		}
		var id, sellerID, price, compareAt, stock int64
		var status string
		var hasCompareAt bool
		err = h.queryRow(r, `
			SELECT p.id,p.seller_id,COALESCE(s.display_name,''),p.name,p.slug,p.brand_slug,b.name,p.category_slug,c.name,p.description,
			       COALESCE(v.sku,''),COALESCE(v.price_cents,p.price_cents),COALESCE(v.compare_at_cents,0),v.compare_at_cents IS NOT NULL,
			       COALESCE(i.available_quantity,0),p.delivery_label,p.status,
			       COALESCE(NULLIF(p.image_url,''),(SELECT pm.public_url FROM product_media pm WHERE pm.product_id=p.id AND pm.status='ready' AND pm.content_type LIKE 'image/%' ORDER BY pm.id DESC LIMIT 1),'')
			FROM products p LEFT JOIN sellers s ON s.id=p.seller_id
			JOIN brands b ON b.slug=p.brand_slug JOIN categories c ON c.slug=p.category_slug
			LEFT JOIN LATERAL (SELECT id,sku,price_cents,compare_at_cents FROM product_variants WHERE product_id=p.id AND status='active' ORDER BY id LIMIT 1) v ON TRUE
			LEFT JOIN inventory_stock i ON i.variant_id=v.id WHERE p.id=$1`, productID).Scan(&id, &sellerID, &form.SellerName, &form.Name, &form.Slug, &form.BrandSlug, &form.BrandName, &form.CategorySlug, &form.CategoryName, &form.Description, &form.SKU, &price, &compareAt, &hasCompareAt, &stock, &form.DeliveryLabel, &status, &form.Image)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "product details unavailable", http.StatusServiceUnavailable)
			return
		}
		if errors.Is(err, pgx.ErrNoRows) || status == "archived" {
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
		productMediaPreviewURLs(form.Media)
		form.Image = productImagePreviewURL(form.Image)
		page.Title = "Edit product"
		page.Subtitle = "Update all listing details. Stock remains in the permission-scoped Inventory workflow."
	}
	form.FeaturedRank = "1000"
	if form.ID != "" {
		if err := h.queryRow(r, `SELECT featured_rank::text,featured_pinned,array_to_string(search_tags,', ') FROM products WHERE id=$1`, form.ID).Scan(&form.FeaturedRank, &form.FeaturedPinned, &form.SearchTags); err != nil {
			http.Error(w, "Product curation unavailable", 503)
			return
		}
	}
	categoryRows, err := h.query(r, `SELECT c.slug,c.name,EXISTS(SELECT 1 FROM product_categories pc WHERE pc.category_slug=c.slug AND pc.product_id=NULLIF($1,'')::bigint) FROM categories c ORDER BY c.display_order,c.slug`, form.ID)
	if err != nil {
		http.Error(w, "Categories unavailable", 503)
		return
	}
	for categoryRows.Next() {
		var category viewmodels.Category
		if err := categoryRows.Scan(&category.Slug, &category.Name, &category.Assigned); err != nil {
			categoryRows.Close()
			http.Error(w, "Categories unavailable", 503)
			return
		}
		form.Categories = append(form.Categories, category)
	}
	err = categoryRows.Err()
	categoryRows.Close()
	if err != nil {
		http.Error(w, "Categories unavailable", 503)
		return
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
	if requestedWorkspace := r.URL.Query().Get("workspace"); requestedWorkspace == "settlements" && (role == domainidentity.RoleAdmin || role == domainidentity.RoleSuperAdmin) {
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
	case "roles", "users", "security-events":
		active = "security"
	case "audit-logs", "system-health", "search-indexing-health", "worker-queue-health", "settings":
		active = "analytics"
	case "seller-approvals", "seller-details", "seller-suspensions":
		active = "sellers"
	case "coupons":
		active = "marketing"
	}
	if domainidentity.CompanyRole(role) && workspace == "refunds" {
		active = "finance"
	}
	return h.adminData(r, actorID, active, workspace, r.URL.Query().Get("notice"), r.URL.Query().Get("q"))
}

func (h *LivePortalHandler) adminData(r *http.Request, actorID int64, active, workspace, notice, query string) viewmodels.AdminPage {
	r = projectionRequest(r)
	page := viewmodels.AdminPage{ActorID: actorID, Active: active, Workspace: workspace, Title: title(active), Subtitle: "Live marketplace operations backed by PostgreSQL.", Notice: notice, Query: query, Range: "Last 30 Days", OrderStage: orderStageFilter(r)}
	if workspace == "" {
		page.Title = "Marketplace dashboard"
	}
	if workspace == "reviews" {
		page.Title = "Review moderation"
		page.Subtitle = "Moderate verified customer reviews and publish approved feedback."
	}
	if workspace == "users" {
		page.Title = "User Management"
		page.Subtitle = "Create, search, update, suspend and remove marketplace accounts with an audited workflow."
	}
	if workspace == "users" {
		page.RoleAssignments = h.adminUsers(r, query)
		page.Sellers = h.adminSellers(r)
		page.Pagination = projectionPagination(r, "admin-users")
		page.DataWarning = projectionWarning(r, page.DataWarning)
		return page
	}
	page.Stats = h.adminStats(r)

	page.Sellers = h.adminSellers(r)
	page.Products = h.adminProducts(r, portalQuery(query))
	if page.Products == nil {
		page.DataWarning = "The product list could not be loaded. Refresh the page to try again."
	}
	page.Orders = h.adminOrders(r)
	page.Returns = h.adminReturns(r)
	page.Transactions = h.adminTransactions(r)
	page.SupportCases = h.adminCases(r)
	if page.SupportCases == nil {
		page.DataWarning = joinPortalWarning(page.DataWarning, "The support queue could not be loaded. Refresh the page to try again.")
	}
	page.Reviews = h.adminReviews(r)
	page.Campaigns = h.adminCampaigns(r)
	page.Settlements = h.adminSettlements(r)
	if h.hasRole(r, actorID, "super_admin") {
		page.AuditEntries = h.adminAudit(r)
		page.Configs = h.runtimeConfigs(r)
	}
	page.Pagination = projectionPagination(r, adminProjectionKey(active, workspace))
	page.DataWarning = projectionWarning(r, page.DataWarning)
	if page.DataWarning != "" {
		page.Stats = nil
	}
	if active == "security" && h.roleAdmin != nil {
		assignments, err := h.roleAdmin.List(r.Context(), actorID, query)
		if err != nil {
			page.DataWarning = joinPortalWarning(page.DataWarning, "Role assignments could not be loaded.")
			page.Stats = nil
		} else {
			page.RoleAssignments = assignments
		}
	}
	return page
}

func (h *LivePortalHandler) adminStats(r *http.Request) []viewmodels.AdminStat {
	var sellers, products, orders, returns, tickets int64
	_ = h.queryRow(r, `SELECT COUNT(*) FROM sellers`).Scan(&sellers)
	_ = h.queryRow(r, `SELECT COUNT(*) FROM products`).Scan(&products)
	_ = h.queryRow(r, `SELECT COUNT(*) FROM orders`).Scan(&orders)
	_ = h.queryRow(r, `SELECT COUNT(*) FROM return_requests WHERE status='pending'`).Scan(&returns)
	_ = h.queryRow(r, `SELECT COUNT(*) FROM support_tickets WHERE status IN ('open','in_progress','waiting_customer')`).Scan(&tickets)
	return []viewmodels.AdminStat{{Icon: "groups", Value: strconv.FormatInt(sellers, 10), Label: "Sellers", Delta: "Live", Positive: true, Period: "database"}, {Icon: "inventory_2", Value: strconv.FormatInt(products, 10), Label: "Products", Delta: "Live", Positive: true, Period: "database"}, {Icon: "shopping_bag", Value: strconv.FormatInt(orders, 10), Label: "Orders", Delta: "Live", Positive: true, Period: "database"}, {Icon: "undo", Value: strconv.FormatInt(returns, 10), Label: "Pending returns", Delta: "Live", Period: "database"}, {Icon: "support_agent", Value: strconv.FormatInt(tickets, 10), Label: "Open support cases", Delta: "Live", Period: "database"}}
}

func (h *LivePortalHandler) portalStats(r *http.Request, role domainidentity.Role) []viewmodels.AdminStat {
	count := func(query string, label, icon string) viewmodels.AdminStat {
		var value int64
		if h.pool != nil {
			_ = h.queryRow(r, query).Scan(&value)
		}
		return viewmodels.AdminStat{Icon: icon, Value: strconv.FormatInt(value, 10), Label: label, Delta: "Live", Positive: value == 0, Period: "current"}
	}
	switch role {
	case domainidentity.RoleAdmin:
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
	rows, err := h.projectionRows(r, "admin-inventory", `
		SELECT pv.id,p.name, pv.sku, COALESCE(stock.available_quantity, 0), COALESCE(stock.reserved_quantity, 0), pv.price_cents, COALESCE(NULLIF(p.image_url, ''), '')
		FROM product_variants pv
		JOIN products p ON p.id = pv.product_id
		LEFT JOIN inventory_stock stock ON stock.variant_id = pv.id
		WHERE pv.status = 'active' AND ($1='' OR concat_ws(' ',p.name,pv.sku,CASE WHEN COALESCE(stock.available_quantity,0)<=5 THEN 'low_stock' ELSE 'in_stock' END) ILIKE '%'||$1||'%')
		ORDER BY p.name, pv.id`, portalQuery(r.URL.Query().Get("q")))
	if err != nil {
		return nil
	}
	defer rows.Close()
	items := make([]viewmodels.AdminInventory, 0)
	for rows.Next() {
		var name, sku, image string
		var variantID int64
		var available, reserved int64
		var price int64
		if rows.Scan(&variantID, &name, &sku, &available, &reserved, &price, &image) != nil {
			return nil
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
			VariantID: strconv.FormatInt(variantID, 10), Name: name, SKU: sku, Stock: strconv.FormatInt(onHand, 10), Reorder: signal,
			Reserved: strconv.FormatInt(reserved, 10), Available: strconv.FormatInt(available, 10),
			Location: "India", Value: money(price * available), Status: status,
			Image: image,
		})
	}
	if rows.Err() != nil {
		return nil
	}
	return items
}

func (h *LivePortalHandler) adminSellers(r *http.Request) []viewmodels.AdminSeller {
	if h.pool == nil {
		return nil
	}
	rows, err := h.projectionRows(r, "admin-sellers", `SELECT id, display_name, status, created_at FROM sellers WHERE status <> 'closed' AND ($1='' OR concat_ws(' ',display_name,status) ILIKE '%'||$1||'%') ORDER BY id DESC`, portalQuery(r.URL.Query().Get("q")))
	if err != nil {
		return nil
	}
	defer rows.Close()
	result := make([]viewmodels.AdminSeller, 0)
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
	rows, err := h.projectionRows(r, "admin-products", `SELECT p.id,p.name,COALESCE(pv.sku,''),COALESCE(s.display_name,''),c.name,COALESCE(pv.price_cents,p.price_cents),p.status,COALESCE(NULLIF(p.image_url,''),(SELECT pm.public_url FROM product_media pm WHERE pm.product_id=p.id AND pm.status='ready' AND pm.content_type LIKE 'image/%' ORDER BY pm.id DESC LIMIT 1),''),p.created_at,COALESCE((SELECT string_agg(sku,' ') FROM product_variants WHERE product_id=p.id AND status='active'),'') FROM products p JOIN categories c ON c.slug=p.category_slug LEFT JOIN sellers s ON s.id=p.seller_id LEFT JOIN LATERAL (SELECT sku,price_cents FROM product_variants WHERE product_id=p.id AND status='active' ORDER BY id LIMIT 1) pv ON TRUE WHERE ($1='' OR p.name ILIKE '%'||$1||'%' OR p.slug ILIKE '%'||$1||'%' OR EXISTS (SELECT 1 FROM product_variants pvx WHERE pvx.product_id=p.id AND pvx.status='active' AND pvx.sku ILIKE '%'||$1||'%') OR COALESCE(s.display_name,'') ILIKE '%'||$1||'%' OR c.name ILIKE '%'||$1||'%' OR p.status ILIKE '%'||$1||'%') ORDER BY p.created_at DESC,p.id DESC`, query)
	if err != nil {
		return nil
	}
	defer rows.Close()
	result := make([]viewmodels.AdminProduct, 0)
	for rows.Next() {
		var id int64
		var name, sku, seller, category, status, image, searchSKUs string
		var submitted time.Time
		var price int64
		if rows.Scan(&id, &name, &sku, &seller, &category, &price, &status, &image, &submitted, &searchSKUs) == nil {
			// Company operators can inspect private product media while a listing
			// is in draft/review. The preview endpoint still enforces the session
			// and product ownership/company-role checks.
			image = productImagePreviewURL(strings.TrimSpace(image))
			result = append(result, viewmodels.AdminProduct{ID: strconv.FormatInt(id, 10), Name: name, SKU: sku, SearchSKUs: searchSKUs, Seller: seller, Category: category, Price: money(price), Status: status, Submitted: submitted.Format("2 Jan 2006"), Image: image})
		}
	}
	if rows.Err() != nil {
		return nil
	}
	return result
}

func (h *LivePortalHandler) adminOrders(r *http.Request) []viewmodels.AdminOrder {
	rows, err := h.projectionRows(r, "admin-orders", `
		SELECT o.order_number,o.created_at,u.display_name,o.total_cents,o.status,
		       COALESCE(pa.status,'unrecorded'),
		       COALESCE(parcel.status,'pending'), COALESCE(parcel.details,'[]'::jsonb)
		FROM orders o JOIN users u ON u.id=o.user_id
		LEFT JOIN LATERAL (SELECT status FROM payment_attempts WHERE order_id=o.id ORDER BY id DESC LIMIT 1) pa ON TRUE
		LEFT JOIN LATERAL (SELECT CASE
				WHEN bool_and(COALESCE(sf.status,'pending')='delivered') THEN 'delivered'
				WHEN bool_and(COALESCE(sf.status,'pending') IN ('out_for_delivery','delivered')) THEN 'out_for_delivery'
				WHEN bool_and(COALESCE(sf.status,'pending') IN ('shipped','out_for_delivery','delivered')) THEN 'shipped'
				WHEN bool_or(COALESCE(sf.status,'pending')='packing') THEN 'packing'
				WHEN bool_or(COALESCE(sf.status,'pending') IN ('processing','shipped','out_for_delivery','delivered')) THEN 'processing'
				ELSE 'pending' END AS status,
 jsonb_agg(DISTINCT jsonb_build_object('seller_name',ss.display_name,'status',COALESCE(sf.status,'pending'),'carrier',COALESCE(sf.carrier,''),'tracking_number',COALESCE(sf.tracking_number,''))) AS details
				FROM order_items soi JOIN product_variants spv ON spv.id=soi.variant_id JOIN products sp ON sp.id=spv.product_id JOIN sellers ss ON ss.id=sp.seller_id LEFT JOIN seller_fulfillments sf ON sf.order_id=soi.order_id AND sf.seller_id=sp.seller_id WHERE soi.order_id=o.id) parcel ON TRUE
		WHERE ($1='' OR concat_ws(' ',o.order_number,u.display_name,o.status,COALESCE(pa.status,'')) ILIKE '%'||$1||'%')
		AND ($2='' OR COALESCE(parcel.status,'pending')=$2)
		ORDER BY o.created_at DESC,o.id DESC`, portalQuery(r.URL.Query().Get("q")), orderStageFilter(r))
	if err != nil {
		return nil
	}
	defer rows.Close()
	result := make([]viewmodels.AdminOrder, 0)
	for rows.Next() {
		var number, customer, status, payment, fulfillment string
		var created time.Time
		var total int64
		var encoded []byte
		var parcels []domaincommerce.OrderFulfillment
		if rows.Scan(&number, &created, &customer, &total, &status, &payment, &fulfillment, &encoded) == nil {
			if json.Unmarshal(encoded, &parcels) != nil {
				continue
			}
			fulfillment = domaincommerce.FulfillmentProgress(parcels)
			result = append(result, viewmodels.AdminOrder{Number: number, Date: created.Format("2 Jan 2006, 15:04"), Customer: customer, Value: money(total), Status: status, PaymentStatus: payment, Fulfillment: fulfillment, Parcels: parcels})
		}
	}
	return result
}

func (h *LivePortalHandler) adminReturns(r *http.Request) []viewmodels.AdminReturn {
	rows, err := h.projectionRows(r, "admin-returns", `SELECT rr.id,o.order_number,u.display_name,rr.reason,rr.status,o.total_cents FROM return_requests rr JOIN orders o ON o.id=rr.order_id JOIN users u ON u.id=rr.user_id WHERE ($1='' OR concat_ws(' ','RTN-'||rr.id::text,o.order_number,u.display_name,rr.reason,rr.status) ILIKE '%'||$1||'%') ORDER BY rr.created_at DESC,rr.id DESC`, portalQuery(r.URL.Query().Get("q")))
	if err != nil {
		return nil
	}
	defer rows.Close()
	result := make([]viewmodels.AdminReturn, 0)
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
	rows, err := h.projectionRows(r, "admin-transactions", `SELECT pa.created_at,pa.status,o.order_number,pa.amount_cents,pa.provider FROM payment_attempts pa JOIN orders o ON o.id=pa.order_id WHERE ($1='' OR concat_ws(' ',o.order_number,'Payment',pa.provider,pa.status) ILIKE '%'||$1||'%') AND ($2='' OR pa.status=$2) ORDER BY pa.created_at DESC,pa.id DESC`, portalQuery(r.URL.Query().Get("q")), transactionProjectionStatus(r))
	if err != nil {
		return nil
	}
	defer rows.Close()
	result := make([]viewmodels.AdminTransaction, 0)
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
	rows, err := h.projectionRows(r, "admin-cases", `SELECT t.ticket_number,u.display_name,t.subject,t.priority,t.status,t.created_at FROM support_tickets t JOIN users u ON u.id=t.user_id WHERE ($1='' OR concat_ws(' ',t.ticket_number,u.display_name,t.subject,t.priority,t.status) ILIKE '%'||$1||'%') ORDER BY t.created_at DESC,t.id DESC`, portalQuery(r.URL.Query().Get("q")))
	if err != nil {
		return nil
	}
	defer rows.Close()
	result := make([]viewmodels.AdminSupportCase, 0)
	for rows.Next() {
		var number, customer, subject, priority, status string
		var created time.Time
		if err := rows.Scan(&number, &customer, &subject, &priority, &status, &created); err != nil {
			return nil
		}
		result = append(result, viewmodels.AdminSupportCase{Case: number, Customer: customer, Subject: subject, Priority: priority, Status: status, Date: created.Format("2 Jan 2006")})
	}
	if rows.Err() != nil {
		return nil
	}
	return result
}

func (h *LivePortalHandler) adminReviews(r *http.Request) []viewmodels.AdminReview {
	rows, err := h.projectionRows(r, "admin-reviews", `
		SELECT pr.id, pr.product_id, p.name, COALESCE(s.display_name,''), u.display_name,
		       pr.title, pr.body, pr.status, pr.rating, pr.verified_purchase, pr.created_at
		FROM product_reviews pr
		JOIN products p ON p.id=pr.product_id
		JOIN users u ON u.id=pr.user_id
		LEFT JOIN sellers s ON s.id=p.seller_id
		WHERE ($1='' OR concat_ws(' ',p.name,s.display_name,u.display_name,pr.title,pr.body,pr.status) ILIKE '%'||$1||'%')
		ORDER BY CASE pr.status WHEN 'pending' THEN 0 ELSE 1 END, pr.created_at DESC, pr.id DESC`, portalQuery(r.URL.Query().Get("q")))
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
	return path == "/admin/marketing/create" || path == "/admin/marketing/update" || path == "/admin/marketing/status" || path == "/admin/marketing/delete" ||
		path == "/super-admin/marketing/create" || path == "/super-admin/marketing/update" || path == "/super-admin/marketing/status" || path == "/super-admin/marketing/delete"
}

func (h *LivePortalHandler) adminPromotionMutation(w http.ResponseWriter, r *http.Request) {
	requiredRole := "admin"
	if strings.HasPrefix(r.URL.Path, "/super-admin/") {
		requiredRole = "super_admin"
	}
	session, ok := h.session(r)
	if !ok {
		redirectToLogin(w, r)
		return
	}
	if r.Method != http.MethodPost || !h.hasRole(r, session.UserID, requiredRole) {
		denyPortalAccess(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid promotion request", http.StatusBadRequest)
		return
	}
	if subtle.ConstantTimeCompare([]byte(session.CSRFToken), []byte(r.Form.Get("csrf_token"))) != 1 {
		denyPortalAccess(w, r)
		return
	}
	http.Error(w, "Promotion publication required. Manage coupon definitions in Storefront → Promotions and submit for Super Admin review.", http.StatusConflict)

}

type promotionForm struct {
	name, description, code, kind, slug, start, end, targetingMode string
	value, limit                                                   int64
	productIDs                                                     []int64
}

func requestID(r *http.Request) string {
	if value := strings.TrimSpace(r.Header.Get("X-Request-ID")); value != "" {
		return value
	}
	return "unassigned"
}

func promotionFormFromRequest(r *http.Request) promotionForm {
	form := promotionForm{name: strings.TrimSpace(r.Form.Get("name")), description: strings.TrimSpace(r.Form.Get("description")), code: strings.ToUpper(strings.TrimSpace(r.Form.Get("code"))), kind: strings.TrimSpace(r.Form.Get("discount_type")), start: strings.TrimSpace(r.Form.Get("starts_on")), end: strings.TrimSpace(r.Form.Get("ends_on")), targetingMode: strings.TrimSpace(r.Form.Get("targeting_mode"))}
	for _, raw := range r.Form["product_ids"] {
		id, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
		if err == nil && id > 0 && !containsInt64(form.productIDs, id) {
			form.productIDs = append(form.productIDs, id)
		}
	}
	return form
}

func containsInt64(values []int64, wanted int64) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func readPromotionForm(r *http.Request) (promotionForm, error) {
	f := promotionFormFromRequest(r)
	if f.targetingMode == "" {
		f.targetingMode = "all"
	}
	if f.targetingMode != "all" && f.targetingMode != "selected" {
		return f, fmt.Errorf("choose whether this coupon applies to all products or selected products")
	}
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
	// The slug is an internal stable identifier. Names are intentionally
	// repeatable, so add a monotonic suffix for each new campaign.
	f.slug = fmt.Sprintf("%s-%d", f.slug, time.Now().UnixNano())
	if err = tx.QueryRow(r.Context(), `INSERT INTO promotions(slug,name,description,discount_type,discount_value,status,starts_at,ends_at,usage_limit,targeting_mode,created_by) VALUES($1,$2,$3,$4,$5,'draft',$6,$7,NULLIF($8,0),$9,$10) RETURNING id`, f.slug, f.name, f.description, f.kind, f.value, start, end, f.limit, f.targetingMode, actorID).Scan(&id); err != nil {
		return err
	}
	if _, err = tx.Exec(r.Context(), `INSERT INTO coupons(promotion_id,code,usage_limit) VALUES($1,$2,NULLIF($3,0))`, id, f.code, f.limit); err != nil {
		return err
	}
	if err = savePromotionTargeting(r.Context(), tx, id, f); err != nil {
		return err
	}
	if _, err = tx.Exec(r.Context(), `INSERT INTO audit_logs(actor_id,action,resource_type,resource_id,request_id,metadata) VALUES($1,'promotion.create','promotion',$2,$3,jsonb_build_object('targeting_mode',$4::text))`, strconv.FormatInt(actorID, 10), strconv.FormatInt(id, 10), requestID(r), f.targetingMode); err != nil {
		return err
	}
	return tx.Commit(r.Context())
}

func (h *LivePortalHandler) updatePromotion(r *http.Request, id, actorID int64) error {
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
	result, err := tx.Exec(r.Context(), `UPDATE promotions SET name=$1,description=$2,discount_type=$3,discount_value=$4,starts_at=$5,ends_at=$6,usage_limit=NULLIF($7,0),targeting_mode=$8,updated_at=NOW() WHERE id=$9`, f.name, f.description, f.kind, f.value, start, end, f.limit, f.targetingMode, id)
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
	if err = savePromotionTargeting(r.Context(), tx, id, f); err != nil {
		return err
	}
	if _, err = tx.Exec(r.Context(), `INSERT INTO audit_logs(actor_id,action,resource_type,resource_id,request_id,metadata) VALUES($1,'promotion.update','promotion',$2,$3,jsonb_build_object('targeting_mode',$4::text))`, strconv.FormatInt(actorID, 10), strconv.FormatInt(id, 10), requestID(r), f.targetingMode); err != nil {
		return err
	}
	return tx.Commit(r.Context())
}

func savePromotionTargeting(ctx context.Context, tx pgx.Tx, promotionID int64, f promotionForm) error {
	if _, err := tx.Exec(ctx, `DELETE FROM promotion_products WHERE promotion_id=$1`, promotionID); err != nil {
		return err
	}
	if f.targetingMode != "selected" {
		return nil
	}
	for _, productID := range f.productIDs {
		result, err := tx.Exec(ctx, `INSERT INTO promotion_products(promotion_id,product_id) SELECT $1,id FROM products WHERE id=$2 ON CONFLICT DO NOTHING`, promotionID, productID)
		if err != nil {
			return err
		}
		if result.RowsAffected() == 0 {
			return fmt.Errorf("selected product %d is no longer available", productID)
		}
	}
	return nil
}

func promotionDraftFromForm(f promotionForm, id int64, r *http.Request) viewmodels.PromotionDraft {
	value := strings.TrimSpace(r.Form.Get("discount_value"))
	if value == "" && f.value > 0 {
		if f.kind == "fixed" {
			value = strconv.FormatFloat(float64(f.value)/100, 'f', 2, 64)
		} else {
			value = strconv.FormatInt(f.value, 10)
		}
	}
	selected := make([]string, 0, len(f.productIDs))
	for _, id := range f.productIDs {
		selected = append(selected, strconv.FormatInt(id, 10))
	}
	draftID := ""
	if id > 0 {
		draftID = strconv.FormatInt(id, 10)
	}
	return viewmodels.PromotionDraft{ID: draftID, Name: f.name, Description: f.description, Code: f.code, Type: f.kind, Value: value, StartsOn: f.start, EndsOn: f.end, UsageLimit: strings.TrimSpace(r.Form.Get("usage_limit")), TargetingMode: f.targetingMode, SelectedProductIDs: selected}
}

func promotionErrorMessage(err error) string {
	message := strings.ToLower(err.Error())
	switch {
	case strings.Contains(message, "coupons_code_lower_key"), strings.Contains(message, "coupons_code_key"), strings.Contains(message, "duplicate key") && strings.Contains(message, "code"):
		return "That coupon code is already in use. Choose a different code."
	case strings.Contains(message, "promotions_slug_key"):
		return "This campaign could not be assigned an internal identifier. Try saving again."
	case strings.Contains(message, "selected product"):
		return "One of the selected products is no longer available. Refresh the catalogue and try again."
	default:
		return "The promotion could not be saved. Check the highlighted values and try again."
	}
}

func (h *LivePortalHandler) renderPromotionFailure(w http.ResponseWriter, r *http.Request, actorID int64, role domainidentity.Role, root, csrf string, err error, form promotionForm, id int64) {
	pageRequest := requestWithPath(r, root+"/marketing")
	page := h.adminPage(pageRequest, actorID, role)
	page.PortalRoot, page.SettlementRoot = root, root+"/finance"
	page.PortalLabel, page.RoleLabel = portalLabel(role), roleLabel(role)
	page.DisplayName, page.CSRFToken = h.userDisplayName(r, actorID), csrf
	page.CanFinance, page.CanSecurity, page.CanManageCatalog = role == domainidentity.RoleAdmin || role == domainidentity.RoleSuperAdmin, role == domainidentity.RoleSuperAdmin, domainidentity.CompanyRole(role)
	page.PromotionError = promotionErrorMessage(err)
	page.PromotionCodeError = strings.Contains(page.PromotionError, "coupon code")
	page.PromotionDraft = promotionDraftFromForm(form, id, r)
	if id > 0 {
		for index := range page.Campaigns {
			if page.Campaigns[index].ID != strconv.FormatInt(id, 10) {
				continue
			}
			draft := page.PromotionDraft
			page.Campaigns[index].Name = draft.Name
			page.Campaigns[index].Description = draft.Description
			page.Campaigns[index].Code = draft.Code
			page.Campaigns[index].Type = draft.Type
			page.Campaigns[index].Value = draft.Value
			page.Campaigns[index].StartsOn = draft.StartsOn
			page.Campaigns[index].EndsOn = draft.EndsOn
			page.Campaigns[index].UsageLimit = draft.UsageLimit
			page.Campaigns[index].TargetingMode = draft.TargetingMode
			page.Campaigns[index].SelectedProductIDs = draft.SelectedProductIDs
		}
		page.PromotionDraft = viewmodels.PromotionDraft{}
	}
	page.Notice = ""
	h.renderAdminComponent(w, r, adminpages.PortalMarketing(page), page)
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
	if !h.hasRole(r, session.UserID, "admin", "super_admin") {
		denyPortalAccess(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid review decision", http.StatusBadRequest)
		return
	}
	token := r.Form.Get("csrf_token")
	if subtle.ConstantTimeCompare([]byte(session.CSRFToken), []byte(token)) != 1 {
		denyPortalAccess(w, r)
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
	rows, err := h.projectionRows(r, "admin-campaigns", `
		SELECT p.id,p.name,p.description,COALESCE(c.code,''),p.discount_type,p.discount_value,p.status,
		       COALESCE(to_char(p.starts_at AT TIME ZONE 'UTC', 'DD Mon YYYY'), 'Not scheduled'),
		       COALESCE(to_char(p.ends_at AT TIME ZONE 'UTC', 'DD Mon YYYY'), 'No end date'),
	       COALESCE(to_char(p.starts_at AT TIME ZONE 'Asia/Kolkata', 'YYYY-MM-DD'),''),
	       COALESCE(to_char(p.ends_at AT TIME ZONE 'Asia/Kolkata' - INTERVAL '1 day', 'YYYY-MM-DD'),''),
	       COUNT(pp.product_id),COALESCE(c.usage_limit,p.usage_limit,0),COALESCE(c.usage_count,0),p.targeting_mode,
	       COALESCE(string_agg(pp.product_id::text, ',' ORDER BY pp.product_id),'')
		FROM promotions p LEFT JOIN promotion_products pp ON pp.promotion_id = p.id
		LEFT JOIN LATERAL (SELECT code,usage_limit,usage_count FROM coupons WHERE promotion_id=p.id ORDER BY id LIMIT 1) c ON TRUE
		WHERE ($1='' OR concat_ws(' ',p.name,p.description,c.code,p.status) ILIKE '%'||$1||'%')
		GROUP BY p.id,c.code,c.usage_limit,c.usage_count ORDER BY p.id DESC`, portalQuery(r.URL.Query().Get("q")))
	if err != nil {
		return nil
	}
	defer rows.Close()
	result := make([]viewmodels.AdminCampaign, 0)
	for rows.Next() {
		var item viewmodels.AdminCampaign
		var value int64
		var productCount int64
		var usageLimit, usageCount int64
		var selectedIDs, targetingMode string
		if rows.Scan(&item.ID, &item.Name, &item.Description, &item.Code, &item.Type, &value, &item.Status, &item.Start, &item.End, &item.StartsOn, &item.EndsOn, &productCount, &usageLimit, &usageCount, &targetingMode, &selectedIDs) == nil {
			item.TargetingMode = targetingMode
			for _, selected := range strings.Split(selectedIDs, ",") {
				if strings.TrimSpace(selected) != "" {
					item.SelectedProductIDs = append(item.SelectedProductIDs, strings.TrimSpace(selected))
				}
			}
			appliesTo := "All eligible products"
			if targetingMode == "selected" {
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
	rows, err := h.projectionRows(r, "admin-settlements", `
		SELECT ss.id,s.display_name,ss.reference,ss.gross_cents,ss.commission_cents,ss.net_cents,ss.status,
		       ss.period_start,ss.period_end,
	       COUNT(DISTINCT sl.order_id) FILTER (WHERE sl.entry_type='sale')
		FROM seller_settlements ss JOIN sellers s ON s.id=ss.seller_id
		LEFT JOIN seller_ledger sl ON sl.settlement_id=ss.id
 WHERE ($1='' OR concat_ws(' ',s.display_name,ss.reference,ss.status) ILIKE '%'||$1||'%')
		GROUP BY ss.id,s.display_name ORDER BY ss.id DESC`, portalQuery(r.URL.Query().Get("q")))
	if err != nil {
		return nil
	}
	defer rows.Close()
	result := make([]viewmodels.AdminSettlement, 0)
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
	return path == "/admin/finance/settlements/create" || path == "/admin/finance/settlements/status" ||
		path == "/super-admin/finance/settlements/create" || path == "/super-admin/finance/settlements/status"
}

func (h *LivePortalHandler) adminSettlementMutation(w http.ResponseWriter, r *http.Request) {
	root, role := "/admin/finance", "admin"
	if strings.HasPrefix(r.URL.Path, "/super-admin/") {
		root, role = "/super-admin/finance", "super_admin"
	}
	session, ok := h.session(r)
	if !ok {
		redirectToLogin(w, r)
		return
	}
	if r.Method != http.MethodPost || !h.hasRole(r, session.UserID, role) {
		denyPortalAccess(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid settlement request", http.StatusBadRequest)
		return
	}
	if subtle.ConstantTimeCompare([]byte(session.CSRFToken), []byte(r.Form.Get("csrf_token"))) != 1 {
		denyPortalAccess(w, r)
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
	validTransition := current == "pending" && status == "approved" ||
		current == "approved" && (status == "paid" || status == "failed")
	if !validTransition {
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
	rows, err := h.projectionRows(r, "admin-audit", `SELECT created_at, COALESCE(actor_id::text,''), action, resource_type, resource_id FROM audit_logs WHERE ($1='' OR concat_ws(' ',actor_id::text,action,resource_type,resource_id) ILIKE '%'||$1||'%') ORDER BY id DESC`, portalQuery(r.URL.Query().Get("q")))
	if err != nil {
		return nil
	}
	defer rows.Close()
	result := make([]viewmodels.AdminAuditEntry, 0)
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
	r = projectionRequest(r)
	page := h.sellerPage(r, userID, isOwner, permissions)
	page.DisplayName = displayName
	page.UserEmail = h.userEmail(r, userID)
	page.DataWarning = projectionWarning(r, page.DataWarning)
	page.Initials = initials(displayName)
	page.CSRFToken = csrfToken
	page.RoleLabel = "Seller"
	page.CanProducts = sellerPortalAllowed("/seller-admin/products", isOwner, permissions)
	page.CanProductWrite = page.SellerStatus == "active" && (isOwner)
	page.CanInventory = sellerPortalAllowed("/seller-admin/inventory", isOwner, permissions)
	page.CanInventoryWrite = isOwner && page.SellerStatus == "active"
	page.CanOrders = sellerPortalAllowed("/seller-admin/orders", isOwner, permissions)
	page.CanFulfill = isOwner && page.SellerStatus == "active"
	page.CanReturns = sellerPortalAllowed("/seller-admin/returns", isOwner, permissions)
	page.CanReports = sellerPortalAllowed("/seller-admin/analytics", isOwner, permissions)
	page.CanManageStore = isOwner
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
	page.DataWarning = projectionWarning(r, page.DataWarning)
	page.Initials = initials(displayName)
	page.CSRFToken = csrfToken
	page.RoleLabel = roleLabel(domainidentity.RoleSeller)
	if isOwner {
		page.RoleLabel = roleLabel(domainidentity.RoleSeller)
	}
	page.CanProducts = true
	page.CanProductWrite = page.SellerStatus == "active" && (isOwner)
	if !page.CanProductWrite {
		http.Redirect(w, r, "/seller-admin/onboarding", http.StatusSeeOther)
		return
	}
	page.CanInventoryWrite = isOwner && page.SellerStatus == "active"
	page.CanInventory = sellerPortalAllowed("/seller-admin/inventory", isOwner, permissions)
	page.CanOrders = sellerPortalAllowed("/seller-admin/orders", isOwner, permissions)
	page.CanFulfill = isOwner && page.SellerStatus == "active"
	page.CanReturns = sellerPortalAllowed("/seller-admin/returns", isOwner, permissions)
	page.CanReports = sellerPortalAllowed("/seller-admin/analytics", isOwner, permissions)
	page.CanManageStore = isOwner

	form := viewmodels.SellerProductForm{Action: "/seller-admin/products", InitialStock: "0", DeliveryLabel: "Delivery to serviceable PIN codes in India", BrandName: page.SellerName, BrandSlug: productSlugForPortal(page.SellerName)}
	if categoryRows, categoryErr := h.query(r, `SELECT slug,name FROM categories ORDER BY name`); categoryErr == nil {
		for categoryRows.Next() {
			var category viewmodels.SellerCategoryChoice
			if categoryRows.Scan(&category.Slug, &category.Name) == nil {
				form.Categories = append(form.Categories, category)
			}
		}
		categoryRows.Close()
	}
	if r.URL.Path != "/seller-admin/products/new" {
		relative := strings.Trim(strings.TrimPrefix(r.URL.Path, "/seller-admin/"), "/")
		idText := strings.TrimSuffix(strings.TrimPrefix(relative, "products/"), "/edit")
		id, err := strconv.ParseInt(idText, 10, 64)
		if err != nil || id <= 0 {
			http.NotFound(w, r)
			return
		}
		var price, compareAt, stock int64
		var cost *int64
		var hasCompareAt bool
		err = h.queryRow(r, `
			SELECT p.status,p.id::text,p.name,p.slug,p.brand_slug,b.name,p.category_slug,c.name,p.description,COALESCE(v.price_cents,p.price_cents),COALESCE(v.compare_at_cents,0),v.compare_at_cents IS NOT NULL,v.cost_price_cents,COALESCE(NULLIF(p.image_url,''),(SELECT pm.public_url FROM product_media pm WHERE pm.product_id=p.id AND pm.status='ready' AND pm.content_type LIKE 'image/%' ORDER BY pm.id DESC LIMIT 1),''),
			       COALESCE(v.sku,''),COALESCE(i.available_quantity,0),p.delivery_label
			FROM products p JOIN brands b ON b.slug=p.brand_slug JOIN categories c ON c.slug=p.category_slug
			LEFT JOIN LATERAL (SELECT id,sku,price_cents,compare_at_cents,cost_price_cents FROM product_variants WHERE product_id=p.id AND status='active' ORDER BY id LIMIT 1) v ON TRUE
			LEFT JOIN inventory_stock i ON i.variant_id=v.id
			WHERE p.id=$1 AND p.status<>'archived' AND p.seller_id IN (
				SELECT s.id FROM sellers s WHERE s.owner_user_id=$2 )`, id, userID).Scan(&form.Status, &form.ID, &form.Name, &form.Slug, &form.BrandSlug, &form.BrandName, &form.CategorySlug, &form.CategoryName, &form.Description, &price, &compareAt, &hasCompareAt, &cost, &form.Image, &form.SKU, &stock, &form.DeliveryLabel)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				http.NotFound(w, r)
			} else {
				http.Error(w, "product details unavailable", http.StatusServiceUnavailable)
			}
			return
		}
		form.PriceCents = strconv.FormatInt(price, 10)
		if cost != nil {
			form.CostPriceCents = strconv.FormatInt(*cost, 10)
		}
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
		productMediaPreviewURLs(form.Media)
		form.Image = productImagePreviewURL(form.Image)
		page.Title = "Edit product draft"
	} else {
		page.Title = "Create product draft"
	}
	applySellerProductInput(r, &form)
	page.Pagination = viewmodels.PortalPagination{}
	page.Subtitle = "Save a product draft, then submit it for marketplace review. Products ship to serviceable PIN codes in India."
	if form.ID != "" {
		if err := h.queryRow(r, `SELECT array_to_string(search_tags,', ') FROM products WHERE id=$1`, form.ID).Scan(&form.SearchTags); err != nil {
			http.Error(w, "Product tags unavailable", 503)
			return
		}
	}
	h.render(w, r, sellerpages.ProductForm(page, form))
}

func productSlugForPortal(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	dash := false
	for _, r := range value {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
			dash = false
		} else if !dash && b.Len() > 0 {
			b.WriteByte('-')
			dash = true
		}
	}
	return strings.Trim(b.String(), "-")
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

func productMediaPreviewURLs(media []viewmodels.ProductMedia) {
	for index := range media {
		media[index].URL = "/media/products/" + media[index].ID + "/preview"
	}
}

func productImagePreviewURL(image string) string {
	if !strings.HasPrefix(image, "/media/products/") || strings.HasSuffix(image, "/preview") {
		return image
	}
	return image + "/preview"
}

func (h *LivePortalHandler) productVariantViews(r *http.Request, productID int64) ([]viewmodels.ProductVariant, error) {
	rows, err := h.query(r, `SELECT pv.id,pv.name,pv.sku,pv.price_cents,COALESCE(pv.compare_at_cents,0),
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
	r = projectionRequest(r)
	path := strings.TrimPrefix(r.URL.Path, "/seller-admin")
	active := strings.Trim(path, "/")
	if active == "" {
		active = "dashboard"
	}
	var sellerID int64
	var name, sellerStatus string
	var lowStockThreshold int
	if err := h.queryRow(r, `SELECT s.id,s.display_name,s.status,s.low_stock_threshold FROM sellers s WHERE s.owner_user_id=$1  ORDER BY CASE WHEN s.owner_user_id=$1 THEN 0 ELSE 1 END,s.id LIMIT 1`, userID).Scan(&sellerID, &name, &sellerStatus, &lowStockThreshold); err != nil {
		return viewmodels.SellerAdminPage{Active: active, Title: "Seller workspace", Subtitle: "No seller profile is assigned to this account.", DataWarning: projectionWarning(r, "")}
	}
	period := r.URL.Query().Get("range")
	if period != "day" && period != "week" && period != "month" {
		period = "week"
	}
	rangeLabel := map[string]string{"day": "Today", "week": "This week", "month": "This month"}[period]
	page := viewmodels.SellerAdminPage{LocalDeliverySimulation: h.localMode, Active: active, Workspace: active, Title: title(active), Subtitle: name + " · seller workspace", Query: r.URL.Query().Get("q"), Tab: r.URL.Query().Get("tab"), Range: period, RangeLabel: rangeLabel, Notice: r.URL.Query().Get("notice"), SellerName: name, SellerStatus: sellerStatus, LowStockThreshold: lowStockThreshold}
	if sellerPortalAllowed("/seller-admin/products", isOwner, permissions) {
		page.Products = h.sellerProducts(r, sellerID, portalQuery(page.Query))
		if page.Products == nil {
			page.DataWarning = joinPortalWarning(page.DataWarning, "The product list could not be loaded. Refresh the page to try again.")
		}
	}
	if sellerPortalAllowed("/seller-admin/inventory", isOwner, permissions) {
		page.Inventory = h.sellerInventory(r, sellerID, portalQuery(page.Query), lowStockThreshold)
		if page.Inventory == nil {
			page.DataWarning = joinPortalWarning(page.DataWarning, "The inventory list could not be loaded. Refresh the page to try again.")
		}
	}
	if sellerPortalAllowed("/seller-admin/orders", isOwner, permissions) {
		page.Orders = h.sellerOrders(r, sellerID)
	}
	if sellerPortalAllowed("/seller-admin/returns", isOwner, permissions) {
		page.Returns = h.sellerReturns(r, sellerID)
	}
	if sellerPortalAllowed("/seller-admin/analytics", isOwner, permissions) {
		page.Stats = h.sellerStats(r, sellerID, period)
		page.TopProducts = h.sellerTopProducts(r, sellerID, period)
	}
	if sellerPortalAllowed("/seller-admin/analytics", isOwner, permissions) {
		page.Ledger = h.sellerLedger(r, sellerID)
		page.Settlements = h.sellerSettlements(r, sellerID)
	}
	if isOwner {
		if active == "settings" {
			_ = h.queryRow(r, `SELECT public_location,public_story,public_technique,public_portrait FROM sellers WHERE id=$1`, sellerID).Scan(&page.PublicLocation, &page.PublicStory, &page.PublicTechnique, &page.PublicPortrait)
		}
	}
	page.InventoryFilter = sellerStockFilter(r.URL.Query().Get("stock"))
	if active == "inventory" {
		if variantID, err := strconv.ParseInt(r.URL.Query().Get("variant"), 10, 64); err == nil && variantID > 0 {
			page.SelectedVariant = strconv.FormatInt(variantID, 10)
			page.InventoryHistory = h.sellerInventoryHistory(r, sellerID, variantID)
		}
	}
	page.Pagination = projectionPagination(r, sellerProjectionKey(active, page.Tab))
	page.LedgerPagination = projectionPagination(r, "seller-ledger")
	page.SettlementPagination = projectionPagination(r, "seller-settlements")
	if active == "earnings" {
		page.Pagination = viewmodels.PortalPagination{}
	}
	page.DataWarning = projectionWarning(r, page.DataWarning)
	if page.DataWarning != "" {
		page.Stats = nil
	}
	for _, item := range page.Inventory {
		if item.IsLowStock {
			page.LowStock = append(page.LowStock, item)
		}
	}
	return page
}

func (h *LivePortalHandler) sellerTopProducts(r *http.Request, sellerID int64, period string) []viewmodels.SellerAdminTopProduct {
	rows, err := h.query(r, `SELECT oi.product_name,oi.sku,SUM(oi.quantity),SUM(oi.line_total_cents),MAX(p.image_url),MAX(v.cost_price_cents),SUM(oi.quantity * COALESCE(v.cost_price_cents,0)) FROM orders o JOIN order_items oi ON oi.order_id=o.id JOIN product_variants v ON v.id=oi.variant_id JOIN products p ON p.id=v.product_id WHERE p.seller_id=$1 AND o.status IN ('paid','processing','shipped','delivered') AND o.created_at >= date_trunc($2,NOW()) GROUP BY oi.product_name,oi.sku ORDER BY SUM(oi.quantity) DESC,SUM(oi.line_total_cents) DESC LIMIT 10`, sellerID, period)
	if err != nil {
		return nil
	}
	defer rows.Close()
	result := make([]viewmodels.SellerAdminTopProduct, 0)
	for rows.Next() {
		var item viewmodels.SellerAdminTopProduct
		var units, revenue, costs int64
		var cost *int64
		if rows.Scan(&item.Name, &item.SKU, &units, &revenue, &item.Image, &cost, &costs) != nil {
			return nil
		}
		item.Units, item.Revenue = strconv.FormatInt(units, 10), money(revenue)
		if cost != nil && units > 0 {
			item.HasCost = true
			unitPrice := revenue / units
			unitProfit := unitPrice - *cost
			item.UnitCost = money(*cost)
			item.UnitProfit = money(unitProfit)
			if unitPrice > 0 {
				item.Margin = fmt.Sprintf("%.1f%%", float64(unitProfit)*100/float64(unitPrice))
			}
		}
		result = append(result, item)
	}
	return result
}

func filterSellerLivePage(page *viewmodels.SellerAdminPage) {
	query := strings.ToLower(strings.TrimSpace(page.Query))
	if query != "" {
		filteredProducts := page.Products[:0]
		for _, item := range page.Products {
			if strings.Contains(strings.ToLower(item.Name+" "+item.SKU+" "+item.SearchSKUs+" "+item.Category+" "+item.Status), query) {
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

func (h *LivePortalHandler) sellerStats(r *http.Request, sellerID int64, period string) []viewmodels.SellerAdminStat {
	var products, orders, revenue, pendingPayout int64
	_ = h.queryRow(r, `SELECT COUNT(*) FROM products WHERE seller_id=$1 AND status<>'archived'`, sellerID).Scan(&products)
	_ = h.queryRow(r, `SELECT COUNT(DISTINCT o.id),COALESCE(SUM(oi.line_total_cents),0)
		FROM orders o JOIN order_items oi ON oi.order_id=o.id JOIN product_variants v ON v.id=oi.variant_id JOIN products p ON p.id=v.product_id
		WHERE p.seller_id=$1 AND o.status IN ('paid','processing','shipped','delivered') AND o.created_at >= date_trunc($2,NOW())`, sellerID, period).Scan(&orders, &revenue)
	_ = h.queryRow(r, `SELECT COALESCE(SUM(net_cents),0) FROM seller_ledger WHERE seller_id=$1 AND status='open' AND entry_type IN ('sale','refund')`, sellerID).Scan(&pendingPayout)
	label := map[string]string{"day": "Today", "week": "This week", "month": "This month"}[period]
	return []viewmodels.SellerAdminStat{
		{Icon: "shopping_bag", Value: strconv.FormatInt(orders, 10), Label: "Orders · " + label, Delta: "Paid orders", Positive: true, Period: label, Tone: "blue"},
		{Icon: "payments", Value: money(revenue), Label: "Revenue · " + label, Delta: "Seller item sales", Positive: true, Period: label, Tone: "green"},
		{Icon: "account_balance_wallet", Value: money(pendingPayout), Label: "Pending payout", Delta: "Open seller ledger", Positive: true, Period: "current", Tone: "orange"},
		{Icon: "category", Value: strconv.FormatInt(products, 10), Label: "Products", Delta: "Active and draft", Positive: true, Period: "current", Tone: "green"},
	}
}

func joinPortalWarning(current, warning string) string {
	if current == "" {
		return warning
	}
	return current + " " + warning
}

func (h *LivePortalHandler) sellerProducts(r *http.Request, sellerID int64, query string) []viewmodels.SellerAdminProduct {
	rows, err := h.projectionRows(r, "seller-products", `SELECT p.id,p.name,COALESCE(v.sku,''),c.name,COALESCE(v.price_cents,p.price_cents),p.status,COALESCE(stock.available,0),(SELECT COUNT(*) FROM product_variants pv WHERE pv.product_id=p.id AND pv.status='active'),COALESCE(NULLIF(p.image_url,''),(SELECT pm.public_url FROM product_media pm WHERE pm.product_id=p.id AND pm.status='ready' AND pm.content_type LIKE 'image/%' ORDER BY pm.id DESC LIMIT 1),''),COALESCE((SELECT string_agg(sku,' ') FROM product_variants WHERE product_id=p.id AND status='active'),'') FROM products p JOIN categories c ON c.slug=p.category_slug LEFT JOIN LATERAL (SELECT id,sku,price_cents FROM product_variants WHERE product_id=p.id AND status='active' ORDER BY id LIMIT 1) v ON TRUE LEFT JOIN LATERAL (SELECT SUM(st.available_quantity) AS available FROM product_variants av JOIN inventory_stock st ON st.variant_id=av.id WHERE av.product_id=p.id AND av.status='active') stock ON TRUE WHERE p.seller_id=$1 AND ($2='' OR p.name ILIKE '%'||$2||'%' OR p.slug ILIKE '%'||$2||'%' OR EXISTS (SELECT 1 FROM product_variants pvx WHERE pvx.product_id=p.id AND pvx.status='active' AND pvx.sku ILIKE '%'||$2||'%') OR c.name ILIKE '%'||$2||'%' OR p.status ILIKE '%'||$2||'%') AND ($3='' OR ($3='stock' AND COALESCE(stock.available,0)=0) OR p.status=$3) ORDER BY p.created_at DESC,p.id DESC`, sellerID, query, sellerProductProjectionStatus(r))
	if err != nil {
		return nil
	}
	defer rows.Close()
	result := make([]viewmodels.SellerAdminProduct, 0)
	for rows.Next() {
		var id int64
		var name, sku, category, status, image, searchSKUs string
		var price, stock int64
		var variants int
		if rows.Scan(&id, &name, &sku, &category, &price, &status, &stock, &variants, &image, &searchSKUs) != nil {
			return nil
		}
		image = productImagePreviewURL(strings.TrimSpace(image))
		result = append(result, viewmodels.SellerAdminProduct{ID: strconv.FormatInt(id, 10), Name: name, SKU: sku, SearchSKUs: searchSKUs, Category: category, Price: money(price), Stock: strconv.FormatInt(stock, 10), Status: status, Variants: strconv.Itoa(variants), Availability: "D", Image: image})
	}
	if rows.Err() != nil {
		return nil
	}
	return result
}

func (h *LivePortalHandler) sellerInventory(r *http.Request, sellerID int64, query string, threshold int) []viewmodels.SellerAdminInventory {
	rows, err := h.projectionRows(r, "seller-inventory", `SELECT v.id,p.name,v.sku,COALESCE(v.name,'Default variant'),COALESCE(i.available_quantity,0),COALESCE(i.reserved_quantity,0) FROM products p JOIN product_variants v ON v.product_id=p.id LEFT JOIN inventory_stock i ON i.variant_id=v.id WHERE p.seller_id=$1 AND v.status='active' AND ($3='' OR ($3='low' AND COALESCE(i.available_quantity,0)<$4) OR ($3='out' AND COALESCE(i.available_quantity,0)=0)) AND ($2='' OR p.name ILIKE '%'||$2||'%' OR v.sku ILIKE '%'||$2||'%' OR COALESCE(v.name,'') ILIKE '%'||$2||'%') ORDER BY p.id DESC,v.id DESC`, sellerID, query, sellerStockFilter(r.URL.Query().Get("stock")), threshold)
	if err != nil {
		return nil
	}
	defer rows.Close()
	result := make([]viewmodels.SellerAdminInventory, 0)
	for rows.Next() {
		var variantID int64
		var name, sku, variant string
		var available, reserved int64
		if rows.Scan(&variantID, &name, &sku, &variant, &available, &reserved) != nil {
			return nil
		}
		status := "In stock"
		if available == 0 {
			status = "Out of stock"
		} else if available < int64(threshold) {
			status = "Low stock"
		}
		result = append(result, viewmodels.SellerAdminInventory{VariantID: strconv.FormatInt(variantID, 10), Name: name, SKU: sku, Variant: variant, Location: "India", Available: strconv.FormatInt(available, 10), Reserved: strconv.FormatInt(reserved, 10), Reorder: strconv.Itoa(threshold), Status: status, IsLowStock: available < int64(threshold), Image: "/assets/images/customer/madhubani-tree.webp"})
	}
	if rows.Err() != nil {
		return nil
	}
	return result
}

func (h *LivePortalHandler) sellerOrders(r *http.Request, sellerID int64) []viewmodels.SellerAdminOrder {
	rows, err := h.projectionRows(r, "seller-orders", `
		SELECT o.order_number, o.created_at, u.display_name, COALESCE(SUM(oi.line_total_cents),0),
		       COALESCE(sf.status,'pending'), COALESCE(pa.status, o.status), COALESCE(SUM(oi.quantity),0), COALESCE(sf.revision,0)
		FROM orders o
		JOIN users u ON u.id=o.user_id
		JOIN order_items oi ON oi.order_id=o.id
		JOIN product_variants pv ON pv.id=oi.variant_id
		JOIN products p ON p.id=pv.product_id AND p.seller_id=$1
		LEFT JOIN seller_fulfillments sf ON sf.order_id=o.id AND sf.seller_id=$1
		LEFT JOIN LATERAL (SELECT status FROM payment_attempts WHERE order_id=o.id ORDER BY id DESC LIMIT 1) pa ON TRUE
		WHERE o.status IN ('paid','processing','shipped','delivered','pending_payment') AND ($2='' OR concat_ws(' ',o.order_number,u.display_name,COALESCE(sf.status,'pending'),COALESCE(pa.status,o.status)) ILIKE '%'||$2||'%' OR EXISTS(SELECT 1 FROM order_items search_item JOIN product_variants search_variant ON search_variant.id=search_item.variant_id JOIN products search_product ON search_product.id=search_variant.product_id WHERE search_item.order_id=o.id AND search_product.seller_id=$1 AND concat_ws(' ',search_item.product_name,search_item.sku) ILIKE '%'||$2||'%')) AND ($3='' OR COALESCE(sf.status,'pending')=$3)
		GROUP BY o.id, u.display_name, sf.status, sf.revision, pa.status
		ORDER BY o.created_at DESC, o.id DESC`, sellerID, portalQuery(r.URL.Query().Get("q")), sellerOrderProjectionStatus(r))
	if err != nil {
		return nil
	}
	defer rows.Close()
	result := make([]viewmodels.SellerAdminOrder, 0)
	for rows.Next() {
		var number, customer, status, payment string
		var created time.Time
		var total, items, revision int64
		if rows.Scan(&number, &created, &customer, &total, &status, &payment, &items, &revision) == nil {
			result = append(result, viewmodels.SellerAdminOrder{SellerID: strconv.FormatInt(sellerID, 10), Number: number, Date: created.Format("2 Jan 2006"), Customer: customer, Items: strconv.FormatInt(items, 10), Value: money(total), Type: "Domestic", Status: status, NextStatus: nextFulfillmentStatus(status), Revision: strconv.FormatInt(revision, 10), Payment: displayPaymentStatus(payment, ""), Image: "/assets/images/customer/madhubani-tree.webp"})
		}
	}
	return result
}

func nextFulfillmentStatus(status string) string {
	stage, ok := domaincommerce.ParseFulfillmentStage(status)
	if !ok {
		return ""
	}
	return string(stage.Next())
}

func (h *LivePortalHandler) sellerReturns(r *http.Request, sellerID int64) []viewmodels.SellerAdminReturn {
	rows, err := h.projectionRows(r, "seller-returns", `
		SELECT rr.id,o.order_number,u.display_name,rr.reason,rr.created_at,rr.status,o.total_cents,
		       COALESCE(string_agg(DISTINCT p.name, ', '),'')
		FROM return_requests rr
		JOIN orders o ON o.id=rr.order_id
		JOIN users u ON u.id=rr.user_id
		JOIN order_items oi ON oi.order_id=o.id
		JOIN product_variants v ON v.id=oi.variant_id
		JOIN products p ON p.id=v.product_id AND p.seller_id=$1
 WHERE ($2='' OR concat_ws(' ','RTN-'||rr.id::text,o.order_number,u.display_name,rr.reason,rr.status,p.name) ILIKE '%'||$2||'%')
		GROUP BY rr.id,o.order_number,u.display_name,rr.reason,rr.created_at,rr.status,o.total_cents
		ORDER BY rr.created_at DESC`, sellerID, portalQuery(r.URL.Query().Get("q")))
	if err != nil {
		return nil
	}
	defer rows.Close()
	result := make([]viewmodels.SellerAdminReturn, 0)
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
	rows, err := h.projectionRows(r, "seller-ledger", `SELECT sl.id,sl.created_at,sl.gross_cents,sl.commission_cents,sl.net_cents,sl.status,sl.entry_type,COALESCE(o.order_number,'') FROM seller_ledger sl LEFT JOIN orders o ON o.id=sl.order_id WHERE sl.seller_id=$1 AND ($2='' OR concat_ws(' ',COALESCE(o.order_number,'LEDGER-'||sl.id::text),sl.status,sl.entry_type) ILIKE '%'||$2||'%') ORDER BY sl.id DESC`, sellerID, portalQuery(r.URL.Query().Get("q")))
	if err != nil {
		return nil
	}
	defer rows.Close()
	result := make([]viewmodels.SellerAdminLedger, 0)
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
	rows, err := h.projectionRows(r, "seller-settlements", `SELECT reference,net_cents,status,period_end FROM seller_settlements WHERE seller_id=$1 AND ($2='' OR concat_ws(' ',reference,status) ILIKE '%'||$2||'%') ORDER BY id DESC`, sellerID, portalQuery(r.URL.Query().Get("q")))
	if err != nil {
		return nil
	}
	defer rows.Close()
	result := make([]viewmodels.SellerAdminSettlement, 0)
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

func portalQuery(value string) string {
	value = strings.TrimSpace(value)
	runes := []rune(value)
	if len(runes) > 120 {
		return string(runes[:120])
	}
	return value
}

func (h *LivePortalHandler) renderSupport(w http.ResponseWriter, r *http.Request, actorID int64, csrfToken string) {
	r = projectionRequest(r)
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
	r = projectionRequest(r)
	active := "dashboard"
	trimmed := strings.Trim(strings.TrimPrefix(r.URL.Path, "/support-portal"), "/")
	if trimmed != "" {
		active = trimmed
	}
	page := viewmodels.SupportPortalPage{Active: active, Title: title(active), Subtitle: "Support cases backed by live PostgreSQL records.", Query: r.URL.Query().Get("q"), Notice: r.URL.Query().Get("notice"), Tab: r.URL.Query().Get("tab"), DisplayName: h.userDisplayName(r, actorID), RoleLabel: roleLabel(domainidentity.RoleAdmin), CSRFToken: csrfToken}
	page.Initials = initials(page.DisplayName)
	page.Cases = h.supportCases(r)

	page.Conversations = h.latestSupportConversations(r)
	if strings.EqualFold(active, "cases") && (len(page.Cases) > 0 || r.URL.Query().Get("ticket_id") != "") {
		selectedID, _ := strconv.ParseInt(r.URL.Query().Get("ticket_id"), 10, 64)
		for _, item := range page.Cases {
			if selectedID == 0 || item.DatabaseID == selectedID {
				page.SelectedCase = item
				break
			}
		}
		if selectedID > 0 && page.SelectedCase.DatabaseID == 0 {
			selectedRequest := r.Clone(r.Context())
			selectedURL := *r.URL
			selectedRequest.URL = &selectedURL
			values := selectedURL.Query()
			values.Set("q", "")
			values.Set("selected_case_id", strconv.FormatInt(selectedID, 10))
			values.Del("page")
			selectedRequest.URL.RawQuery = values.Encode()
			if cases := h.supportCases(selectedRequest); len(cases) > 0 {
				page.SelectedCase = cases[0]
			}
		}
		if page.SelectedCase.DatabaseID > 0 {
			page.SelectedCase.Attachments = h.supportAttachmentViews(r, page.SelectedCase.DatabaseID)
			page.Messages = append(page.Messages, h.supportTicketMessages(r, page.SelectedCase.DatabaseID)...)
		}
	}

	if strings.EqualFold(page.Active, "customers") {
		page.Directory = h.supportDirectory(r)
	}
	var open, urgent, closed int64
	_ = h.queryRow(r, `SELECT COUNT(*) FILTER (WHERE status NOT IN ('resolved','closed')),COUNT(*) FILTER (WHERE priority='urgent'),COUNT(*) FILTER (WHERE status IN ('resolved','closed')) FROM support_tickets`).Scan(&open, &urgent, &closed)
	page.Stats = []viewmodels.SupportPortalStat{{Icon: "support_agent", Value: strconv.FormatInt(open, 10), Label: "Open tickets", Period: "PostgreSQL"}, {Icon: "warning", Value: strconv.FormatInt(urgent, 10), Label: "Urgent tickets", Period: "PostgreSQL"}, {Icon: "task_alt", Value: strconv.FormatInt(closed, 10), Label: "Resolved or closed", Period: "PostgreSQL"}}
	page.Pagination = projectionPagination(r, "support-cases")
	if active == "customers" {
		page.Pagination = projectionPagination(r, "support-directory")
	}
	page.DataWarning = projectionWarning(r, page.DataWarning)
	if page.DataWarning != "" {
		page.Stats = nil
	}

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

func (h *LivePortalHandler) supportAttachmentViews(r *http.Request, ticketID int64) []viewmodels.SupportPortalAttachment {
	if h.supportAttachments == nil || ticketID <= 0 {
		return nil
	}
	attachments, err := h.supportAttachments.List(r.Context(), ticketID)
	if err != nil {
		return nil
	}
	result := make([]viewmodels.SupportPortalAttachment, 0, len(attachments))
	for _, attachment := range attachments {
		result = append(result, viewmodels.SupportPortalAttachment{
			ID: attachment.ID, FileName: attachment.FileName, ContentType: attachment.ContentType,
			Size: attachment.ByteSize, URL: "/support/attachments/" + strconv.FormatInt(attachment.ID, 10),
		})
	}
	return result
}

func (h *LivePortalHandler) supportCases(r *http.Request) []viewmodels.SupportPortalCase {
	projectionKey := "support-cases"
	selectedID, _ := strconv.ParseInt(r.URL.Query().Get("selected_case_id"), 10, 64)
	if selectedID > 0 {
		projectionKey = "support-selected"
	}
	rows, err := h.projectionRows(r, projectionKey, `SELECT t.id,t.ticket_number,t.subject,t.user_id,u.display_name,t.priority,t.created_at,t.status,t.message,t.agent_note,COALESCE(o.order_number,''),COALESCE(o.status,''),COALESCE(o.total_cents,0),COALESCE(pa.status,''),COALESCE(reply.body,'') FROM support_tickets t JOIN users u ON u.id=t.user_id LEFT JOIN orders o ON o.id=t.order_id LEFT JOIN LATERAL (SELECT status FROM payment_attempts WHERE order_id=o.id ORDER BY id DESC LIMIT 1) pa ON TRUE LEFT JOIN LATERAL (SELECT body FROM support_ticket_messages WHERE ticket_id=t.id AND author_role IN ('admin','super_admin','support_agent') ORDER BY created_at DESC,id DESC LIMIT 1) reply ON TRUE WHERE ($1='' OR concat_ws(' ',t.ticket_number,t.subject,u.display_name,COALESCE(o.order_number,''),t.status,t.priority) ILIKE '%'||$1||'%') AND ($2<>'orders' OR t.order_id IS NOT NULL) AND ($3=0 OR t.id=$3) ORDER BY t.created_at DESC,t.id DESC`, portalQuery(r.URL.Query().Get("q")), strings.Trim(strings.TrimPrefix(r.URL.Path, "/support-portal"), "/"), selectedID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	result := make([]viewmodels.SupportPortalCase, 0)
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
	rows, err := h.query(r, `SELECT m.author_id,u.display_name,m.author_role,m.body,m.created_at FROM support_ticket_messages m JOIN users u ON u.id=m.author_id WHERE m.ticket_id=$1 ORDER BY m.created_at,m.id`, ticketID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	messages := make([]viewmodels.SupportPortalConversation, 0)
	for rows.Next() {
		var authorID int64
		var author, authorRole, body string
		var created time.Time
		if rows.Scan(&authorID, &author, &authorRole, &body, &created) == nil {
			role, tone := "Customer", "customer"
			if authorRole == "admin" {
				role, tone = "Company support", "agent"
			} else {
				author = maskName(author)
				if authorRole == "seller" {
					role = "Seller"
				}
			}
			messages = append(messages, viewmodels.SupportPortalConversation{TicketID: ticketID, Author: author, Initials: initials(author), Role: role, Message: body, Time: created.Format("2 Jan 2006 15:04"), Tone: tone})
		}
	}
	return messages
}

func (h *LivePortalHandler) latestSupportConversations(r *http.Request) []viewmodels.SupportPortalConversation {
	rows, err := h.query(r, `SELECT m.ticket_id,t.ticket_number,u.display_name,m.body,m.created_at FROM support_ticket_messages m JOIN support_tickets t ON t.id=m.ticket_id JOIN users u ON u.id=m.author_id WHERE m.author_role IN ('admin','super_admin','support_agent') ORDER BY m.created_at DESC,m.id DESC LIMIT 12`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	messages := make([]viewmodels.SupportPortalConversation, 0)
	for rows.Next() {
		var item viewmodels.SupportPortalConversation
		var created time.Time
		if rows.Scan(&item.TicketID, &item.TicketNumber, &item.Author, &item.Message, &created) == nil {
			item.Initials = initials(item.Author)
			item.Role = "Company support"
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
	if projectionWarning(r, "") != "" {
		w.WriteHeader(http.StatusServiceUnavailable)
	}
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
		financeTarget = "/access-unavailable"
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
	if page.DataWarning != "" {
		w.WriteHeader(http.StatusServiceUnavailable)
	}
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
		if contains(item.Name, item.SKU, item.SearchSKUs, item.Seller, item.Category, item.Status) {
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

func sellerProductProjectionStatus(r *http.Request) string {
	switch r.URL.Query().Get("tab") {
	case "review", "approval":
		return "pending_review"
	case "draft", "approved", "rejected", "archived", "stock":
		return r.URL.Query().Get("tab")
	case "changes":
		return "changes_required"
	}
	return ""
}

func (h *LivePortalHandler) supportDirectory(r *http.Request) []viewmodels.SupportPortalDirectoryEntry {
	rows, err := h.projectionRows(r, "support-directory", `SELECT u.id,u.display_name,COUNT(DISTINCT t.id),COUNT(DISTINCT o.id),MAX(t.created_at) FROM users u JOIN support_tickets t ON t.user_id=u.id LEFT JOIN orders o ON o.user_id=u.id WHERE ($1='' OR concat_ws(' ',u.display_name,t.ticket_number,t.subject,t.status) ILIKE '%'||$1||'%') GROUP BY u.id ORDER BY MAX(t.created_at) DESC,u.id DESC`, portalQuery(r.URL.Query().Get("q")))
	if err != nil {
		return nil
	}
	defer rows.Close()
	result := make([]viewmodels.SupportPortalDirectoryEntry, 0)
	for rows.Next() {
		var id, cases, orders int64
		var name string
		var latest time.Time
		if rows.Scan(&id, &name, &cases, &orders, &latest) != nil {
			return nil
		}
		result = append(result, viewmodels.SupportPortalDirectoryEntry{Name: maskName(name), EmailPhone: "Contact details masked", Type: "Customer", Orders: strconv.FormatInt(orders, 10), Cases: strconv.FormatInt(cases, 10), Status: "Has support ticket", LastActive: latest.Format("2 Jan 2006 15:04")})
	}
	return result
}
