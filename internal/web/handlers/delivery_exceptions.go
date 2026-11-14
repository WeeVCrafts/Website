package handlers

import (
	"context"
	"crypto/subtle"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	domainidentity "github.com/wecratfs/commerce/internal/domain/identity"
	"github.com/wecratfs/commerce/internal/ports"
	"github.com/wecratfs/commerce/internal/web/viewmodels"
	adminpages "github.com/wecratfs/commerce/web/admin"
)

type deliveryExceptionOperations interface {
	ListDeliveryExceptions(context.Context, int64, string) ([]ports.DeliveryException, error)
	UpdateDeliveryException(context.Context, int64, int64, int64, string, string, string, string) error
}

func (h *LivePortalHandler) SetDeliveryExceptions(repo deliveryExceptionOperations) {
	h.deliveryExceptions = repo
}

func (h *LivePortalHandler) serveDeliveryExceptions(w http.ResponseWriter, r *http.Request) bool {
	root, role := "/admin", domainidentity.RoleAdmin
	if strings.HasPrefix(r.URL.Path, "/super-admin/") {
		root, role = "/super-admin", domainidentity.RoleSuperAdmin
	}
	path := root + "/delivery-exceptions"
	if r.URL.Path != path {
		return false
	}
	w.Header().Set("Cache-Control", "no-store")
	session, ok := h.session(r)
	if !ok {
		redirectToLogin(w, r)
		return true
	}
	if !h.hasRole(r, session.UserID, string(role)) {
		denyPortalAccess(w, r)
		return true
	}
	if h.deliveryExceptions == nil || h.shipping == nil {
		http.Error(w, "Delivery operations are unavailable.", 503)
		return true
	}

	if r.Method == http.MethodPost {
		r.Body = http.MaxBytesReader(w, r.Body, 8192)
		var parseErr error
		if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
			parseErr = r.ParseMultipartForm(8192)
			if r.MultipartForm != nil {
				defer r.MultipartForm.RemoveAll()
			}
		} else {
			parseErr = r.ParseForm()
		}
		if parseErr != nil {
			http.Error(w, "Invalid delivery request.", 400)
			return true
		}
		if session.CSRFToken == "" || subtle.ConstantTimeCompare([]byte(session.CSRFToken), []byte(r.PostForm.Get("csrf_token"))) != 1 {
			denyPortalAccess(w, r)
			return true
		}
		f := r.PostForm
		order, _ := strconv.ParseInt(f.Get("order"), 10, 64)
		seller, _ := strconv.ParseInt(f.Get("seller"), 10, 64)
		action := f.Get("action")
		var err error
		if action == "cancel" {
			ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
			defer cancel()
			err = h.shipping.Cancel(ctx, session.UserID, order, seller)
		} else {
			err = h.deliveryExceptions.UpdateDeliveryException(r.Context(), session.UserID, order, seller, h.shipping.Environment(), f.Get("kind"), action, f.Get("reason"))
		}
		switch {
		case errors.Is(err, ports.ErrForbidden):
			denyPortalAccess(w, r)
		case errors.Is(err, ports.ErrShippingPending):
			http.Redirect(w, r, path+"?notice="+url.QueryEscape("Awaiting verified Shiprocket status. Check the queue before taking further action."), http.StatusSeeOther)
		case errors.Is(err, ports.ErrInvalidState), errors.Is(err, ports.ErrShippingInput), errors.Is(err, ports.ErrShippingRejected):
			http.Error(w, "This action is unavailable. Reload the queue and recheck the shipment stage.", http.StatusConflict)
		case err != nil:
			http.Error(w, "Delivery operations are temporarily unavailable.", http.StatusServiceUnavailable)
		default:
			http.Redirect(w, r, path+"?notice="+url.QueryEscape("Delivery case updated."), http.StatusSeeOther)
		}
		return true
	}
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "Method not allowed.", 405)
		return true
	}
	cases, err := h.deliveryExceptions.ListDeliveryExceptions(r.Context(), session.UserID, h.shipping.Environment())
	if err != nil {
		if errors.Is(err, ports.ErrForbidden) {
			denyPortalAccess(w, r)
		} else {
			http.Error(w, "The delivery queue is temporarily unavailable.", 503)
		}
		return true
	}
	page := viewmodels.AdminPage{Active: "orders", NavActive: "orders", Title: "Delivery exceptions", PortalRoot: root, PortalLabel: portalLabel(role), RoleLabel: roleLabel(role), DisplayName: h.userDisplayName(r, session.UserID), CSRFToken: session.CSRFToken, CanFinance: true, CanManageCatalog: true, ActorID: session.UserID, CanSecurity: role == domainidentity.RoleSuperAdmin, Notice: r.URL.Query().Get("notice")}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = adminpages.DeliveryExceptions(page, cases).Render(r.Context(), w)
	return true
}
