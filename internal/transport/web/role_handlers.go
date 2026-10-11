package web

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	applicationauth "github.com/wecratfs/commerce/internal/application/auth"
	"github.com/wecratfs/commerce/internal/identity"
	"github.com/wecratfs/commerce/internal/ports"
)

func (h *Handler) adminRoles(w http.ResponseWriter, r *http.Request) {
	if h.roleAdminService == nil || r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	session, ok := h.sessionFromRequest(r)
	if !ok {
		h.redirect(w, r, "/login")
		return
	}
	assignments, err := h.roleAdminService.List(r.Context(), session.UserID, r.URL.Query().Get("q"))
	if errors.Is(err, ports.ErrForbidden) {
		h.renderError(w, http.StatusForbidden, "Role administration restricted", "Only Super Admin can change operational role assignments.")
		return
	}
	if err != nil {
		h.renderError(w, http.StatusServiceUnavailable, "Role administration unavailable", "We could not load the live role assignments.")
		return
	}
	h.render(w, http.StatusOK, "roles", pageData{
		Brand: identity.Name, Tagline: identity.Tagline, Description: identity.Description,
		Title: "Role management · " + identity.Name, Authenticated: true, AccountUserID: session.UserID,
		CSRFToken: session.CSRFToken, RoleAssignments: assignments,
	})
}

func (h *Handler) adminRoleUpdate(w http.ResponseWriter, r *http.Request) {
	if h.roleAdminService == nil || r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	session, ok := h.sessionFromRequest(r)
	if !ok {
		h.redirect(w, r, "/login")
		return
	}
	if !h.validSessionCSRF(r, session) {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	userID, err := strconv.ParseInt(r.FormValue("user_id"), 10, 64)
	if err != nil || userID <= 0 {
		h.renderError(w, http.StatusUnprocessableEntity, "Role change invalid", "Choose a valid user.")
		return
	}
	grant := r.FormValue("action") == "grant"
	if r.FormValue("action") != "grant" && r.FormValue("action") != "revoke" {
		h.renderError(w, http.StatusUnprocessableEntity, "Role change invalid", "Choose grant or revoke.")
		return
	}
	sellerID := int64(0)
	if raw := strings.TrimSpace(r.FormValue("seller_id")); raw != "" {
		parsed, parseErr := strconv.ParseInt(raw, 10, 64)
		if parseErr != nil || parsed <= 0 {
			h.renderError(w, http.StatusUnprocessableEntity, "Role change invalid", "Choose an active seller for seller staff access.")
			return
		}
		sellerID = parsed
	}
	err = h.roleAdminService.Update(r.Context(), session.UserID, userID, sellerID, r.FormValue("role"), grant, r.FormValue("reason"))
	if err != nil {
		switch {
		case errors.Is(err, ports.ErrForbidden):
			h.renderError(w, http.StatusForbidden, "Role administration restricted", "Only Super Admin can change operational roles.")
		case errors.Is(err, ports.ErrNotFound):
			h.renderError(w, http.StatusNotFound, "User not found", "That user account is no longer available.")
		case errors.Is(err, ports.ErrSellerNotFound):
			h.renderError(w, http.StatusUnprocessableEntity, "Seller not available", "Choose an active seller before granting Seller Staff access.")
		case errors.Is(err, ports.ErrRoleState), errors.Is(err, applicationauth.ErrInvalidRoleAssignment):
			h.renderError(w, http.StatusConflict, "Role change rejected", "The role assignment is invalid, unchanged, or protected.")
		default:
			h.renderError(w, http.StatusServiceUnavailable, "Role change unavailable", "We could not save the role assignment.")
		}
		return
	}
	h.redirect(w, r, "/super-admin/users?notice=Role+updated")
}

func (h *Handler) adminRoleUserCreate(w http.ResponseWriter, r *http.Request) {
	if h.roleAdminService == nil || r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	session, ok := h.sessionFromRequest(r)
	if !ok {
		h.redirect(w, r, "/login")
		return
	}
	if !h.validSessionCSRF(r, session) {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	sellerID := int64(0)
	if raw := strings.TrimSpace(r.FormValue("seller_id")); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || parsed <= 0 {
			h.renderError(w, http.StatusUnprocessableEntity, "User details invalid", "Choose an active seller for seller staff accounts.")
			return
		}
		sellerID = parsed
	}
	err := h.roleAdminService.CreateUser(r.Context(), session.UserID, r.FormValue("email"), r.FormValue("display_name"), r.FormValue("password"), r.FormValue("role"), sellerID, r.FormValue("reason"))
	switch {
	case err == nil:
		h.redirect(w, r, "/super-admin/users?notice=User+account+created+and+role+assigned")
	case errors.Is(err, ports.ErrForbidden):
		h.renderError(w, http.StatusForbidden, "User management restricted", "Only Super Admin can create operational accounts.")
	case errors.Is(err, ports.ErrUserAlreadyExists):
		h.renderError(w, http.StatusConflict, "Email already in use", "An account already uses that email address.")
	case errors.Is(err, ports.ErrSellerNotFound):
		h.renderError(w, http.StatusUnprocessableEntity, "Seller not available", "Choose an active seller before assigning the Seller Staff role.")
	case errors.Is(err, applicationauth.ErrInvalidRoleAssignment):
		h.renderError(w, http.StatusUnprocessableEntity, "User details invalid", "Check the email, password, role, seller selection and audit reason.")
	default:
		h.renderError(w, http.StatusServiceUnavailable, "User creation unavailable", "We could not create this account.")
	}
}

func (h *Handler) adminRoleUserDelete(w http.ResponseWriter, r *http.Request) {
	if h.roleAdminService == nil || r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	session, ok := h.sessionFromRequest(r)
	if !ok {
		h.redirect(w, r, "/login")
		return
	}
	if !h.validSessionCSRF(r, session) {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	userID, err := strconv.ParseInt(r.FormValue("user_id"), 10, 64)
	if err != nil || userID <= 0 {
		h.renderError(w, http.StatusUnprocessableEntity, "User request invalid", "Choose a valid user account.")
		return
	}
	err = h.roleAdminService.DeleteUser(r.Context(), session.UserID, userID, r.FormValue("reason"))
	switch {
	case err == nil:
		h.redirect(w, r, "/super-admin/users?notice=User+account+deleted")
	case errors.Is(err, ports.ErrForbidden):
		h.renderError(w, http.StatusForbidden, "User management restricted", "Only Super Admin can delete operational accounts.")
	case errors.Is(err, ports.ErrNotFound):
		h.renderError(w, http.StatusNotFound, "User not found", "That user account is no longer available.")
	case errors.Is(err, applicationauth.ErrInvalidRoleAssignment), errors.Is(err, ports.ErrRoleState):
		h.renderError(w, http.StatusConflict, "User deletion rejected", "The request is invalid or would remove the last Super Admin account.")
	default:
		h.renderError(w, http.StatusServiceUnavailable, "User deletion unavailable", "We could not delete this account.")
	}
}

func (h *Handler) adminRoleUserUpdate(w http.ResponseWriter, r *http.Request) {
	if h.roleAdminService == nil || r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	session, ok := h.sessionFromRequest(r)
	if !ok {
		h.redirect(w, r, "/login")
		return
	}
	if !h.validSessionCSRF(r, session) {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	userID, err := strconv.ParseInt(r.FormValue("user_id"), 10, 64)
	if err != nil || userID <= 0 {
		h.renderError(w, 422, "Invalid user", "Choose a valid account.")
		return
	}
	err = h.roleAdminService.UpdateUser(r.Context(), session.UserID, userID, r.FormValue("email"), r.FormValue("display_name"), r.FormValue("status"), r.FormValue("password"), r.FormValue("reason"))
	switch {
	case err == nil:
		h.redirect(w, r, "/super-admin/users?notice=Account+updated")
	case errors.Is(err, ports.ErrForbidden):
		h.renderError(w, 403, "User management restricted", "Only Super Admin can edit accounts.")
	case errors.Is(err, ports.ErrUserAlreadyExists):
		h.renderError(w, 409, "Email already in use", "Choose a different email address.")
	case errors.Is(err, ports.ErrNotFound):
		h.renderError(w, 404, "User not found", "This account no longer exists.")
	case errors.Is(err, ports.ErrRoleState):
		h.renderError(w, 409, "Account protected", "You cannot suspend yourself, edit a deleted account, or disable the last active Super Admin.")
	case errors.Is(err, applicationauth.ErrInvalidRoleAssignment):
		h.renderError(w, 422, "Invalid account details", "Check the email, name, status and audit reason. New passwords need at least 12 characters.")
	default:
		h.renderError(w, 503, "Account update failed", "Please try again.")
	}
}
