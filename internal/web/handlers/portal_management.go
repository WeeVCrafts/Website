package handlers

import (
	"crypto/subtle"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/wecratfs/commerce/internal/adapters/postgres"
	applicationcommerce "github.com/wecratfs/commerce/internal/application/commerce"
	domainidentity "github.com/wecratfs/commerce/internal/domain/identity"
	"github.com/wecratfs/commerce/internal/ports"
	"github.com/wecratfs/commerce/internal/web/viewmodels"
	adminpages "github.com/wecratfs/commerce/web/admin"
)

func managementPortalPath(path string) (root, section, action string, ok bool) {
	for _, prefix := range []string{"/super-admin/", "/marketplace-admin/", "/finance-portal/"} {
		if !strings.HasPrefix(path, prefix) {
			continue
		}
		parts := strings.Split(strings.TrimPrefix(path, prefix), "/")
		if len(parts) > 2 {
			return "", "", "", false
		}
		section = parts[0]
		if prefix == "/finance-portal/" && section != "commission-rules" {
			return "", "", "", false
		}
		switch section {
		case "customers", "brands", "category-management", "commission-rules", "settings":
		default:
			return "", "", "", false
		}
		if len(parts) == 2 {
			action = parts[1]
			if action != "create" && action != "update" && action != "delete" && action != "save" {
				return "", "", "", false
			}
		}
		return strings.TrimSuffix(prefix, "/"), section, action, true
	}
	return "", "", "", false
}

// serveManagement handles the live administrative directory and configuration workflows.
func (h *LivePortalHandler) serveManagement(w http.ResponseWriter, r *http.Request) bool {
	root, section, action, handled := managementPortalPath(r.URL.Path)
	if !handled {
		return false
	}
	session, ok := h.session(r)
	if !ok {
		redirectToLogin(w, r)
		return true
	}
	role := domainidentity.RoleMarketplaceAdmin
	if root == "/super-admin" {
		role = domainidentity.RoleSuperAdmin
	} else if root == "/finance-portal" {
		role = domainidentity.RoleFinanceOperator
	}
	if !h.hasRole(r, session.UserID, roleString(role)) {
		denyPortalAccess(w, r)
		return true
	}
	if role == domainidentity.RoleMarketplaceAdmin && section == "commission-rules" {
		http.Error(w, "finance access required", http.StatusForbidden)
		return true
	}
	service := applicationcommerce.NewManagementService(postgres.NewManagementRepository(h.pool))
	if action != "" {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", "POST")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return true
		}
		r.Body = http.MaxBytesReader(w, r.Body, 32<<10)
		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid form", http.StatusBadRequest)
			return true
		}
		token := r.Form.Get("csrf_token")
		if token == "" || subtle.ConstantTimeCompare([]byte(token), []byte(session.CSRFToken)) != 1 {
			http.Error(w, "invalid CSRF token", http.StatusForbidden)
			return true
		}
		points := 0
		if section == "commission-rules" && action != "delete" {
			var err error
			points, err = strconv.Atoi(r.Form.Get("basis_points"))
			if err != nil {
				http.Error(w, "Commission must be an integer from 0 to 10000 basis points.", http.StatusBadRequest)
				return true
			}
		}
		mutation := ports.ManagementMutation{Section: section, Action: action, ID: r.Form.Get("id"), Name: r.Form.Get("name"), Category: r.Form.Get("category"), Reason: r.Form.Get("reason"), BasisPoints: points, Active: r.Form.Get("active") == "true", Settings: ports.StorefrontSettings{SupportEmail: r.Form.Get("support_email"), ShippingInformation: r.Form.Get("shipping_information")}}
		mutation.SaveSocialLinks = r.Form.Get("save_social_links") == "1"
		mutation.Settings.InstagramURL = r.Form.Get("instagram_url")
		mutation.Settings.FacebookURL = r.Form.Get("facebook_url")
		mutation.Settings.YouTubeURL = r.Form.Get("youtube_url")
		mutation.Settings.XURL = r.Form.Get("x_url")
		mutation.Settings.WhatsAppURL = r.Form.Get("whatsapp_url")
		if mutation.SaveSocialLinks && role != domainidentity.RoleSuperAdmin {
			http.Error(w, "Super Admin access required.", 403)
			return true
		}
		if err := service.Mutate(r.Context(), session.UserID, mutation); err != nil {
			switch {
			case errors.Is(err, ports.ErrForbidden):
				http.Error(w, "forbidden", http.StatusForbidden)
			case errors.Is(err, ports.ErrNotFound):
				http.Error(w, "Record no longer exists. Refresh the directory.", http.StatusNotFound)
			case errors.Is(err, ports.ErrManagementValidation):
				http.Error(w, "Invalid input. Check names, slugs, commission rate, customer details and audit reason.", http.StatusBadRequest)
			case errors.Is(err, ports.ErrManagementConflict):
				http.Error(w, "Record already exists or is still referenced. Remove references before deleting. Category slugs must exist.", http.StatusConflict)
			default:
				http.Error(w, "Management change could not be saved.", http.StatusInternalServerError)
			}
			return true
		}
		http.Redirect(w, r, root+"/"+section+"?notice="+url.QueryEscape("Changes saved."), http.StatusSeeOther)
		return true
	}
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return true
	}
	pageNumber, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if pageNumber < 1 || pageNumber > 1000000 {
		pageNumber = 1
	}
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	result, err := service.List(r.Context(), session.UserID, section, query, pageNumber)
	if err != nil {
		if errors.Is(err, ports.ErrForbidden) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return true
		}
		http.Error(w, "Management directory could not be loaded.", http.StatusInternalServerError)
		return true
	}
	page := viewmodels.AdminPage{Active: section, NavActive: section, Workspace: section, Title: title(section), Subtitle: "Manage current marketplace records.", PortalRoot: root, PortalLabel: portalLabel(role), RoleLabel: roleLabel(role), DisplayName: h.userDisplayName(r, session.UserID), CSRFToken: session.CSRFToken, Notice: r.URL.Query().Get("notice"), CanSecurity: role == domainidentity.RoleSuperAdmin, CanFinance: role == domainidentity.RoleSuperAdmin || role == domainidentity.RoleFinanceOperator, CanManageCatalog: role == domainidentity.RoleSuperAdmin || role == domainidentity.RoleMarketplaceAdmin, IsFinance: role == domainidentity.RoleFinanceOperator, Query: query}
	model := viewmodels.ManagementPage{Admin: page, Section: section, Action: root + "/" + section, Query: query, Page: pageNumber, Total: result.Total, Records: result.Records}
	if section == "settings" {
		model.Settings, err = postgres.ReadStorefrontSettings(r.Context(), h.pool)
		if err != nil {
			http.Error(w, "Settings could not be loaded.", http.StatusInternalServerError)
			return true
		}
	}
	pagination := func(number int) string {
		values := url.Values{"q": {query}, "page": {strconv.Itoa(number)}}
		return model.Action + "?" + values.Encode()
	}
	if pageNumber > 1 {
		model.PreviousURL = pagination(pageNumber - 1)
	}
	if pageNumber*25 < result.Total {
		model.NextURL = pagination(pageNumber + 1)
	}
	h.renderAdminComponent(w, r, adminpages.Management(model), page)
	return true
}
