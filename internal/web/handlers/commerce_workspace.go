package handlers

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"github.com/a-h/templ"
	"github.com/wecratfs/commerce/internal/ports"
	"github.com/wecratfs/commerce/internal/web/viewmodels"
	adminpages "github.com/wecratfs/commerce/web/admin"
	"github.com/wecratfs/commerce/web/components"
	"github.com/wecratfs/commerce/web/pages"
	sellerpages "github.com/wecratfs/commerce/web/selleradmin"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

func commerceOrderPath(path string) (prefix, number string, ok bool) {
	for _, base := range []string{"/super-admin", "/seller-admin", "/admin", ""} {
		if n, found := strings.CutPrefix(path, base+"/orders/"); found && n != "" && len(n) <= 120 && !strings.Contains(n, "/") {
			return base, n, true
		}
	}
	return "", "", false
}

func commerceOrderTab(s string) string {
	switch s {
	case "packages", "documents", "returns", "support":
		return s
	}
	return "overview"
}

// Shared entry point for order detail and request queues. Business POSTs remain
// with their existing validated command handlers; new setup POSTs use reviews.
func (h *MarketplaceHandler) serveCommerceSurface(w http.ResponseWriter, r *http.Request) bool {
	prefix, number, isOrder := commerceOrderPath(r.URL.Path)
	isReturns := false
	for _, base := range []string{"/super-admin", "/seller-admin", "/admin", ""} {
		if r.URL.Path == base+"/returns" {
			prefix = base
			isReturns = true
			break
		}
	}
	isSetup := r.URL.Path == "/seller-admin/setup" || r.URL.Path == "/super-admin/fulfillment/reviews" || strings.HasPrefix(r.URL.Path, "/super-admin/fulfillment/reviews/")
	if !isOrder && !isReturns && !isSetup {
		return false
	}
	if h.Repository == nil || h.Sessions == nil {
		http.Error(w, "Order details temporarily unavailable", 503)
		return true
	}
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Robots-Tag", "noindex, nofollow")
	cookie, err := r.Cookie("wecratfs_session")
	if err != nil {
		redirectToLogin(w, r)
		return true
	}
	session, err := h.Sessions.Get(r.Context(), cookie.Value)
	if err != nil {
		redirectToLogin(w, r)
		return true
	}
	a, err := h.Repository.CommerceActor(r.Context(), session.UserID)
	if err != nil {
		marketplaceHTTPError(w, err)
		return true
	}
	if isSetup {
		h.serveCommerceSetup(w, r, session.UserID, session.CSRFToken, a)
		return true
	}
	allowed := prefix == "" && a.Role == "customer" || prefix == "/seller-admin" && a.Role == "seller" || prefix == "/super-admin" && a.Role == "super_admin" || prefix == "/admin" && (a.Role == "admin" || a.Role == "super_admin")
	if !allowed {
		http.Error(w, "Forbidden", 403)
		return true
	}
	if r.Method != "GET" && r.Method != "HEAD" {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", 405)
		return true
	}
	root := prefix + "/fulfillment"
	if prefix == "" {
		root = "/account/fulfillment"
	}
	p := viewmodels.MarketplacePage{MarketplaceView: ports.MarketplaceView{Role: a.Role, Number: number}, Root: root, Back: prefix, ActionPath: root + "/action", CSRFToken: session.CSRFToken, Workspace: true, OrderTab: commerceOrderTab(r.URL.Query().Get("tab")), OrderURL: r.URL.Path, ShippingConfigured: h.ShippingConfigured, BookingsEnabled: h.BookingsEnabled, CheckoutEnabled: h.CheckoutEnabled}
	key := make([]byte, 16)
	if _, err = rand.Read(key); err != nil {
		http.Error(w, "Request unavailable", 503)
		return true
	}
	p.Key = hex.EncodeToString(key)
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	p.RequestSearch = portalQuery(r.URL.Query().Get("q"))
	p.RequestBucket = r.URL.Query().Get("status")
	var body templ.Component
	if isOrder {
		var summary ports.CommerceOrderSummary
		summary, p.MarketplaceView, p.Requests, err = h.Repository.OrderWorkspace(r.Context(), session.UserID, number, ports.RequestQuery{Search: p.RequestSearch, Bucket: p.RequestBucket, Page: page, Focus: r.URL.Query().Get("focus")})
		if err != nil {
			marketplaceHTTPError(w, err)
			return true
		}
		if (a.Role == "seller" || a.Role == "customer") && summary.WorkflowVersion != 2 && r.URL.Query().Get("legacy") == "1" {
			return false
		}
		p.Summary = &summary
		p.Status = summary.Status
		p.Payment = summary.Financial.DisplayStatus
		p.Focus = r.URL.Query().Get("focus")
		if p.OrderTab == "support" && r.URL.Query().Get("case_id") != "" {
			selectedCase, _ := strconv.ParseInt(r.URL.Query().Get("case_id"), 10, 64)
			selected := p.Cases[:0]
			for _, c := range p.Cases {
				if c.ID == selectedCase {
					selected = append(selected, c)
				}
			}
			p.Cases = selected
		}

		p.RequestTask = r.URL.Query().Get("task") == "new"
		p.TaskItemID, _ = strconv.ParseInt(r.URL.Query().Get("item_id"), 10, 64)
		if p.RequestTask {
			selected := p.Lines[:0]
			for _, line := range p.Lines {
				if line.ID == p.TaskItemID {
					selected = append(selected, line)
				}
			}
			p.Lines = selected
			p.Claims = nil
			p.Returns = nil
			p.Refunds = nil
		}
		if p.Focus != "" {
			filterWorkspaceFocus(&p)
		} else if p.OrderTab == "returns" {
			p.Claims = nil
			p.Returns = nil
			p.Refunds = nil
		}
		if r.URL.Query().Get("saved") == "1" {
			p.Notice = "Request saved. Payment and delivery outcomes update after confirmation."
		}
		body = components.CommerceOrderWorkspace(p)
	} else {
		p.Requests, err = h.Repository.CommerceReads().RequestsForActor(r.Context(), session.UserID, ports.RequestQuery{Search: p.RequestSearch, Bucket: p.RequestBucket, Page: page})
		if err != nil {
			marketplaceHTTPError(w, err)
			return true
		}
		body = components.CommerceRequestList(p)
	}
	h.renderCommerceShell(w, r, a, session.CSRFToken, func() string {
		if isReturns {
			return "Returns & refunds"
		}
		return "Order " + number
	}(), func() string {
		if isReturns {
			return "/returns"
		}
		return "orders"
	}(), body)
	return true
}

func filterWorkspaceFocus(p *viewmodels.MarketplacePage) {
	kind, raw, ok := strings.Cut(p.Focus, ":")
	id, err := strconv.ParseInt(raw, 10, 64)
	if !ok || err != nil || id <= 0 {
		p.Focus = ""
		p.Claims = nil
		p.Returns = nil
		p.Refunds = nil
		return
	}
	claims := p.Claims[:0]
	for _, x := range p.Claims {
		if kind == "claim" && x.ID == id {
			claims = append(claims, x)
		}
	}
	p.Claims = claims
	returns := p.Returns[:0]
	for _, x := range p.Returns {
		if kind == "return" && x.ID == id {
			returns = append(returns, x)
		}
	}
	p.Returns = returns
	refunds := p.Refunds[:0]
	for _, x := range p.Refunds {
		if kind == "refund" && x.ID == id {
			refunds = append(refunds, x)
		}
	}
	p.Refunds = refunds
	// New-claim and cancellation forms are deliberately separate tasks.
	if !p.RequestTask {
		p.Lines = nil
	}
}

func (h *MarketplaceHandler) renderCommerceShell(w http.ResponseWriter, r *http.Request, a ports.CommerceActor, csrf, title, active string, body templ.Component) {
	ctx := templ.WithChildren(r.Context(), body)
	var doc templ.Component
	switch a.Role {
	case "seller":
		doc = sellerpages.Document(viewmodels.SellerAdminPage{OwnContentHeading: true, Title: title, Active: active, DisplayName: a.Name, UserEmail: a.Email, SellerName: a.Seller, RoleLabel: "Seller", CSRFToken: csrf, CanOrders: true, CanProducts: true, CanInventory: true, CanReturns: true, CanReports: true, CanManageStore: true})
	case "admin", "super_admin":
		root, label := "/admin", "Admin"
		if a.Role == "super_admin" {
			root, label = "/super-admin", "Super Admin"
		}
		doc = adminpages.AdminDocument(viewmodels.AdminPage{OwnContentHeading: true, Title: title, Active: active, NavActive: active, PortalRoot: root, PortalLabel: label, RoleLabel: label, DisplayName: a.Name, CSRFToken: csrf, CanFinance: true, CanManageCatalog: true, CanSecurity: a.Role == "super_admin"})
	default:
		ctx = templ.WithChildren(r.Context(), pages.CustomerCommerceBody(viewmodels.CustomerPage{UserName: a.Name, UserEmail: a.Email, RoleLabel: "Customer"}, active, body))
		doc = pages.Document(viewmodels.CustomerPage{Title: title, Route: "/orders", Brand: "WeevCrafts", Authenticated: true, IsCustomer: true, CSRFToken: csrf})
	}
	if r.Method != "HEAD" {
		if err := doc.Render(ctx, w); err != nil {
			return
		}
	}
}

func commerceSubmissionID(path string) int64 {
	id, _ := strconv.ParseInt(strings.TrimPrefix(path, "/super-admin/fulfillment/reviews/"), 10, 64)
	return id
}

func commerceSafePost(r *http.Request, csrf string) bool {
	kind, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	var err error
	if kind == "multipart/form-data" {
		err = r.ParseMultipartForm(32768)
	} else {
		err = r.ParseForm()
	}
	if err != nil {
		return false
	}
	return csrf != "" && subtle.ConstantTimeCompare([]byte(csrf), []byte(r.PostForm.Get("csrf_token"))) == 1
}

func decodeSubmissionData(s *ports.SetupSubmission) ports.SetupSubmissionData {
	var d ports.SetupSubmissionData
	if s != nil {
		_ = json.Unmarshal(s.Payload, &d)
	}
	return d
}

func canonicalCommerceURL(role, number, tab string) string {
	prefix := ""
	switch role {
	case "seller":
		prefix = "/seller-admin"
	case "admin":
		prefix = "/admin"
	case "super_admin":
		prefix = "/super-admin"
	}
	return prefix + "/orders/" + url.PathEscape(number) + "?tab=" + commerceOrderTab(tab)
}
