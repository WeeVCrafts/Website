package handlers

import (
	"crypto/rand"
	"crypto/subtle"
	"errors"
	"github.com/wecratfs/commerce/internal/adapters/postgres"
	application "github.com/wecratfs/commerce/internal/application/commerce"
	identity "github.com/wecratfs/commerce/internal/domain/identity"
	"github.com/wecratfs/commerce/internal/ports"
	"github.com/wecratfs/commerce/internal/web/viewmodels"
	adminpages "github.com/wecratfs/commerce/web/admin"
	"net/http"

	"strconv"
	"strings"
)

func (h *LivePortalHandler) confirmationService() *application.SellerConfirmationService {
	return application.NewSellerConfirmationService(postgres.NewCommerceRepository(h.pool), postgres.NewRoleAuthorizer(h.pool))
}

func (h *LivePortalHandler) serveCompanySupport(w http.ResponseWriter, r *http.Request) bool {
	path := r.URL.Path
	root := "/admin"
	if strings.HasPrefix(path, "/super-admin/") {
		root = "/super-admin"
	}
	sellerReply := path == "/seller-admin/support/confirmations/reply"
	if !sellerReply && path != root+"/support" && !strings.HasPrefix(path, root+"/support/") {
		return false
	}
	session, ok := h.session(r)
	if !ok {
		redirectToLogin(w, r)
		return true
	}
	roles := h.userRoles(r, session.UserID)
	role := identity.RoleCustomer
	if len(roles) == 1 {
		role = roles[0]
	}
	if (sellerReply && role != identity.RoleSeller) || (!sellerReply && (!identity.CompanyRole(role) || (root == "/super-admin" && role != identity.RoleSuperAdmin))) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return true
	}
	if h.pool == nil {
		http.Error(w, "support unavailable", http.StatusServiceUnavailable)
		return true
	}
	if r.Method == http.MethodGet && !sellerReply {
		if path == root+"/support/orders" || path == root+"/support/customers" {
			redirectPortalDestination(w, r, root+"/"+strings.TrimPrefix(path, root+"/support/"))
			return true
		}
		if path != root+"/support" && path != root+"/support/cases" {
			http.NotFound(w, r)
			return true
		}
		support := h.supportPage(requestWithPath(r, "/support-portal/cases"), session.UserID, session.CSRFToken)
		if r.URL.Query().Get("ticket_id") != "" && support.SelectedCase.DatabaseID == 0 {
			http.Error(w, "support case not found", http.StatusNotFound)
			return true
		}
		if r.URL.Query().Get("ticket_id") == "" {
			support.SelectedCase = viewmodels.SupportPortalCase{}
		}
		page := viewmodels.CompanySupportPage{Support: support, RequestKey: rand.Text()}
		page.Admin = viewmodels.AdminPage{ActorID: session.UserID, Active: "support", NavActive: "support", Title: "Support & delivery", Subtitle: "Review the case, confirm with the seller and keep the customer informed.", PortalRoot: root, PortalLabel: portalLabel(role), RoleLabel: roleLabel(role), DisplayName: h.userDisplayName(r, session.UserID), CSRFToken: session.CSRFToken, CanFinance: true, CanManageCatalog: true, CanSecurity: role == identity.RoleSuperAdmin, Pagination: support.Pagination, Notice: r.URL.Query().Get("notice"), DataWarning: support.DataWarning}
		var err error
		before, _ := strconv.ParseInt(r.URL.Query().Get("coordination_before"), 10, 64)
		page.QueueStatus = r.URL.Query().Get("coordination_status")
		page.Confirmations, err = h.confirmationService().Queue(r.Context(), session.UserID, support.SelectedCase.DatabaseID, false, page.QueueStatus, before)
		if len(page.Confirmations) > 50 {
			page.Confirmations = page.Confirmations[:50]
			values := r.URL.Query()
			values.Set("coordination_before", strconv.FormatInt(page.Confirmations[49].ID, 10))
			page.QueueNext = root + "/support?" + values.Encode()
		}
		if err == nil && support.SelectedCase.DatabaseID > 0 {
			page.Sellers, err = h.confirmationService().CaseSellers(r.Context(), session.UserID, support.SelectedCase.DatabaseID)
		}
		if errors.Is(err, application.ErrInvalidSupportTicket) {
			http.Error(w, "Invalid request status or cursor", http.StatusBadRequest)
			return true
		}
		if err != nil {
			http.Error(w, "private seller requests could not be loaded", http.StatusServiceUnavailable)
			return true
		}
		h.render(w, r, adminpages.CompanySupport(page))
		return true
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return true
	}
	// Existing customer replies and internal notes retain their authoritative service rules.
	if path == root+"/support/cases/status" || path == root+"/support/cases/reply" {
		action := strings.TrimPrefix(path, root+"/support/cases/")
		h.delegateWithPortalRedirect(w, requestWithPath(r, "/support/"+action), root+"/support")
		return true
	}
	r.Body = http.MaxBytesReader(w, r.Body, 32<<10)
	var parseErr error
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		parseErr = r.ParseMultipartForm(32 << 10)
		if r.MultipartForm != nil {
			defer r.MultipartForm.RemoveAll()
		}
	} else {
		parseErr = r.ParseForm()
	}
	if parseErr != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return true
	}
	token := r.Form.Get("csrf_token")
	if token == "" || session.CSRFToken == "" || subtle.ConstantTimeCompare([]byte(token), []byte(session.CSRFToken)) != 1 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return true
	}
	id, _ := strconv.ParseInt(r.Form.Get("confirmation_id"), 10, 64)
	ticket, _ := strconv.ParseInt(r.Form.Get("ticket_id"), 10, 64)
	var err error
	switch {
	case sellerReply:
		err = h.confirmationService().Reply(r.Context(), session.UserID, id, r.Form.Get("message"), r.Form.Get("request_key"))
	case path == root+"/support/confirmations/create":
		seller, _ := strconv.ParseInt(r.Form.Get("seller_id"), 10, 64)
		_, err = h.confirmationService().Create(r.Context(), session.UserID, ticket, seller, r.Form.Get("message"), r.Form.Get("request_key"))
	case path == root+"/support/confirmations/close":
		err = h.confirmationService().Close(r.Context(), session.UserID, id, r.Form.Get("reason"))
	default:
		http.NotFound(w, r)
		return true
	}
	if err != nil {
		if errors.Is(err, ports.ErrIdempotencyConflict) {
			http.Error(w, "This submission was already used for a different message. Reload the case before trying again.", http.StatusConflict)
		} else if errors.Is(err, application.ErrInvalidSupportTicket) {
			http.Error(w, "Use a message of 1–4000 characters, a unique submission key, and an outcome of 4–1000 characters when closing a request.", http.StatusUnprocessableEntity)
		} else {
			sellerSupportError(w, err)
		}
		return true
	}
	destination := root + "/support?notice=Seller+request+saved"
	if ticket > 0 {
		destination += "&ticket_id=" + strconv.FormatInt(ticket, 10)
	}
	if sellerReply {
		destination = "/seller-admin/support?notice=Confirmation+sent"
	}
	http.Redirect(w, r, destination, http.StatusSeeOther)
	return true
}
