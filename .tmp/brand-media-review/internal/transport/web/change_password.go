package web

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"

	applicationauth "github.com/wecratfs/commerce/internal/application/auth"
)

func (h *Handler) changePassword(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	respond := func(status int, message, redirect string) {
		if r.Header.Get("Accept") == "application/json" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(status)
			_ = json.NewEncoder(w).Encode(map[string]string{"message": message, "redirect": redirect})
			return
		}
		if redirect != "" {
			h.redirect(w, r, redirect)
		} else {
			h.redirect(w, r, "/account/security?notice="+url.QueryEscape(message))
		}
	}
	session, ok := h.sessionFromRequest(r)
	if !ok {
		respond(http.StatusUnauthorized, "Sign in again to change your password.", "/login?next=%2Faccount%2Fsecurity")
		return
	}
	if len(session.RoleSlugs) != 1 || session.RoleSlugs[0] != "customer" {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	if !h.validSessionCSRF(r, session) {
		respond(http.StatusForbidden, "Your form expired. Refresh the page before trying again.", "")
		return
	}
	if h.auth == nil {
		respond(http.StatusServiceUnavailable, "Password changes are temporarily unavailable.", "")
		return
	}
	err := h.auth.ChangePassword(r.Context(), session.UserID, r.FormValue("current_password"), r.FormValue("new_password"), r.FormValue("confirm_password"))
	if err != nil {
		message := "Your password could not be changed. Try again."
		status := http.StatusServiceUnavailable
		switch {
		case errors.Is(err, applicationauth.ErrInvalidCredentials):
			message = "Your current password is incorrect."
			status = http.StatusUnprocessableEntity
		case errors.Is(err, applicationauth.ErrInvalidPasswordChange):
			message = "Use a different password with at least 12 characters and matching confirmation."
			status = http.StatusUnprocessableEntity
		case errors.Is(err, applicationauth.ErrAccountLocked):
			message = "Too many attempts. Wait 15 minutes before trying again."
			status = http.StatusTooManyRequests
		}
		respond(status, message, "")
		return
	}
	// PostgreSQL invalidation remains authoritative even if Redis cleanup fails.
	_ = h.sessions.RevokeAll(r.Context(), session.UserID)
	h.clearSessionCookie(w)
	h.clearCSRFCookie(w)
	respond(http.StatusOK, "Password changed. Sign in again with your new password.", "/login?next=%2Faccount%2Fsecurity&notice=Password+changed.+Sign+in+with+your+new+password.")
}
