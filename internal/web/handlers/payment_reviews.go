package handlers

import (
	"context"
	"crypto/subtle"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	domainidentity "github.com/wecratfs/commerce/internal/domain/identity"
	"github.com/wecratfs/commerce/internal/ports"
	"github.com/wecratfs/commerce/internal/web/viewmodels"
	adminpages "github.com/wecratfs/commerce/web/admin"
)

type paymentReviewOperations interface {
	List(context.Context, int64) ([]ports.PaymentReviewCase, error)
	Approve(context.Context, int64, int64, string) error
	Refund(context.Context, int64, int64, string) error
	RetryRefund(context.Context, int64, int64, string, string) error
}

func (h *LivePortalHandler) SetPaymentReviews(service paymentReviewOperations) {
	h.paymentReviews = service
}

func (h *LivePortalHandler) servePaymentReviews(w http.ResponseWriter, r *http.Request) bool {
	root, role := "/admin", domainidentity.RoleAdmin
	if strings.HasPrefix(r.URL.Path, "/super-admin/") {
		root, role = "/super-admin", domainidentity.RoleSuperAdmin
	}
	path := root + "/payment-reviews"
	if r.URL.Path != path && r.URL.Path != path+"/approve" && r.URL.Path != path+"/refund" && r.URL.Path != path+"/refund-retry" {
		return false
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	session, ok := h.session(r)
	if !ok {
		redirectToLogin(w, r)
		return true
	}
	if !h.hasRole(r, session.UserID, string(role)) {
		denyPortalAccess(w, r)
		return true
	}
	if h.paymentReviews == nil {
		http.Error(w, "Payment reviews are temporarily unavailable", 503)
		return true
	}
	var failure, reason string
	var failedID int64
	if r.Method == http.MethodPost && r.URL.Path != path {
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
			http.Error(w, "Invalid review request", 400)
			return true
		}
		if session.CSRFToken == "" || subtle.ConstantTimeCompare([]byte(session.CSRFToken), []byte(r.Form.Get("csrf_token"))) != 1 {
			denyPortalAccess(w, r)
			return true
		}
		failedID, _ = strconv.ParseInt(r.Form.Get("review_id"), 10, 64)
		reason = strings.TrimSpace(r.Form.Get("reason"))
		var err error
		if strings.HasSuffix(r.URL.Path, "/approve") {
			err = h.paymentReviews.Approve(r.Context(), session.UserID, failedID, reason)
		} else if strings.HasSuffix(r.URL.Path, "/refund-retry") {
			err = h.paymentReviews.RetryRefund(r.Context(), session.UserID, failedID, reason, r.Form.Get("expected_refund_id"))
		} else {
			err = h.paymentReviews.Refund(r.Context(), session.UserID, failedID, reason)
		}
		if err == nil {
			http.Redirect(w, r, path+"?notice="+url.QueryEscape("Decision saved. Review the current case status below."), http.StatusSeeOther)
			return true
		}
		if errors.Is(err, ports.ErrForbidden) {
			denyPortalAccess(w, r)
			return true
		}
		failure = "The decision could not be completed. Check the saved status before retrying. Your reason is retained."
		if errors.Is(err, ports.ErrInsufficientStock) {
			failure = "Stock is unavailable. The captured payment remains held; revalidate stock or authorize its refund."
		}
		if r.Header.Get("X-UI-Form") == "true" {
			http.Error(w, failure, http.StatusConflict)
			return true
		}
		w.WriteHeader(http.StatusConflict)
	} else if r.Method != http.MethodGet || r.URL.Path != path {
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "Method not allowed", 405)
		return true
	}
	cases, err := h.paymentReviews.List(r.Context(), session.UserID)
	if err != nil {
		if errors.Is(err, ports.ErrForbidden) {
			denyPortalAccess(w, r)
		} else {
			http.Error(w, "Payment review list is temporarily unavailable", 503)
		}
		return true
	}
	page := viewmodels.AdminPage{Active: "finance", NavActive: "finance", Title: "Payment reviews", Subtitle: "Resolve captured payments held outside normal checkout. Fulfilment and payout stay blocked while a review is open.", PortalRoot: root, SettlementRoot: root + "/finance", PortalLabel: portalLabel(role), RoleLabel: roleLabel(role), DisplayName: h.userDisplayName(r, session.UserID), CSRFToken: session.CSRFToken, CanFinance: true, CanManageCatalog: true, ActorID: session.UserID, CanSecurity: role == domainidentity.RoleSuperAdmin, Notice: r.URL.Query().Get("notice"), DataWarning: failure}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = adminpages.PaymentReviews(page, cases, failedID, reason).Render(r.Context(), w)
	return true
}
