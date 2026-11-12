package web

import (
	"errors"
	"net/http"
	"strings"

	applicationauth "github.com/wecratfs/commerce/internal/application/auth"
	"github.com/wecratfs/commerce/internal/identity"
	"github.com/wecratfs/commerce/internal/ports"
)

func (h *Handler) forgotPassword(w http.ResponseWriter, r *http.Request) {
	if h.recoveryService == nil {
		h.renderError(w, http.StatusNotFound, "Page not found", "Password recovery is not enabled in this runtime.")
		return
	}
	data := pageData{Brand: identity.Name, Tagline: identity.Tagline, Description: identity.Description, Title: "Recover your password · " + identity.Name}
	if r.Method == http.MethodGet {
		data.CSRFToken = h.ensureCSRFCookie(w, r)
		h.render(w, http.StatusOK, "forgot-password", data)
		return
	}
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if !h.validAnonymousCSRF(w, r) {
		data.CSRFToken = h.ensureCSRFCookie(w, r)
		data.Error = "The form expired. Please try again."
		h.render(w, http.StatusForbidden, "forgot-password", data)
		return
	}
	if err := r.ParseForm(); err != nil {
		data.CSRFToken = h.ensureCSRFCookie(w, r)
		data.Error = "We could not read that form."
		h.render(w, http.StatusBadRequest, "forgot-password", data)
		return
	}
	data.FormEmail = r.FormValue("email")
	if err := h.recoveryService.RequestPasswordReset(r.Context(), data.FormEmail); errors.Is(err, applicationauth.ErrRecoveryInvalidRequest) {
		data.CSRFToken = h.ensureCSRFCookie(w, r)
		data.Error = "Enter a valid email address."
		h.render(w, http.StatusUnprocessableEntity, "forgot-password", data)
		return
	}
	data.CSRFToken = h.ensureCSRFCookie(w, r)
	data.Notice = applicationauth.PasswordResetAcceptedMessage
	h.render(w, http.StatusAccepted, "forgot-password", data)
}

const invalidResetLink = "That reset link is invalid or expired."

func (h *Handler) resetPassword(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Cache-Control", "private, no-store")
	if h.recoveryService == nil {
		h.renderError(w, http.StatusNotFound, "Page not found", "Password recovery is not enabled in this runtime.")
		return
	}
	token := strings.TrimSpace(r.URL.Query().Get("token"))
	if r.Method == http.MethodGet {
		data := recoveryPageData(h, w, r, "Reset your password")
		if token == "" && r.URL.Query().Get("mode") == "resetPassword" && r.URL.Query().Get("oobCode") != "" {
			data.FirebaseAction = true
			h.render(w, http.StatusOK, "reset-password", data)
			return
		}
		active, err := h.recoveryService.PasswordResetLinkActive(r.Context(), token)
		if err != nil {
			data.Error = "We could not check this link. Try again shortly."
			h.render(w, http.StatusServiceUnavailable, "reset-password", data)
			return
		}
		if !active {
			data.Error = invalidResetLink
			status := http.StatusUnprocessableEntity
			if token == "" {
				status = http.StatusOK
			}
			h.render(w, status, "reset-password", data)
			return
		}
		data.ActionToken = token
		h.render(w, http.StatusOK, "reset-password", data)
		return
	}
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if !h.validAnonymousCSRF(w, r) {
		data := recoveryPageData(h, w, r, "Reset your password")
		data.Error = "The form expired. Please try again."
		data.ActionToken = r.FormValue("token")
		h.render(w, http.StatusForbidden, "reset-password", data)
		return
	}
	if err := r.ParseForm(); err != nil {
		h.renderError(w, http.StatusBadRequest, "Reset request invalid", "We could not read that form.")
		return
	}
	token = strings.TrimSpace(r.FormValue("token"))
	data := recoveryPageData(h, w, r, "Reset your password")
	data.ActionToken = token
	password := r.FormValue("password")
	if token == "" {
		data.Error = invalidResetLink
		h.render(w, http.StatusUnprocessableEntity, "reset-password", data)
		return
	}
	if password != r.FormValue("confirm_password") {
		data.Error = "The passwords do not match."
		h.render(w, http.StatusUnprocessableEntity, "reset-password", data)
		return
	}
	userID, err := h.recoveryService.ResetPassword(r.Context(), token, password)
	if err != nil {
		switch {
		case errors.Is(err, applicationauth.ErrRecoveryInvalidRequest):
			data.Error = "Enter a valid token and a password of at least 12 characters."
			h.render(w, http.StatusUnprocessableEntity, "reset-password", data)
		case errors.Is(err, ports.ErrAuthTokenInvalid):
			data.Error = invalidResetLink
			data.ActionToken = ""
			h.render(w, http.StatusUnprocessableEntity, "reset-password", data)
		default:
			h.renderError(w, http.StatusServiceUnavailable, "Password reset unavailable", "We could not reset your password.")
		}
		return
	}
	if h.sessions != nil && userID > 0 {
		// A password reset invalidates every existing browser session. The
		// reset itself remains authoritative even if disposable session
		// infrastructure is temporarily unavailable.
		_ = h.sessions.RevokeAll(r.Context(), userID)
	}
	data.ActionToken = ""
	data.Notice = "Your password has been changed. You can now sign in."
	h.render(w, http.StatusOK, "reset-password", data)
}

func (h *Handler) verifyEmail(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Cache-Control", "private, no-store")
	if h.recoveryService == nil {
		h.renderError(w, http.StatusNotFound, "Page not found", "Email verification is not enabled in this runtime.")
		return
	}
	token := strings.TrimSpace(r.URL.Query().Get("token"))
	if r.Method == http.MethodGet {
		data := recoveryPageData(h, w, r, "Verify your email")
		data.ActionToken = token
		data.FirebaseAction = token == "" && r.URL.Query().Get("mode") == "verifyEmail" && r.URL.Query().Get("oobCode") != ""
		h.render(w, http.StatusOK, "verify-email", data)
		return
	}
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if !h.validAnonymousCSRF(w, r) {
		data := recoveryPageData(h, w, r, "Verify your email")
		data.Error = "The form expired. Please try again."
		data.ActionToken = r.FormValue("token")
		h.render(w, http.StatusForbidden, "verify-email", data)
		return
	}
	if err := r.ParseForm(); err != nil {
		h.renderError(w, http.StatusBadRequest, "Verification request invalid", "We could not read that form.")
		return
	}
	data := recoveryPageData(h, w, r, "Verify your email")
	data.ActionToken = strings.TrimSpace(r.FormValue("token"))
	if _, err := h.recoveryService.VerifyEmail(r.Context(), data.ActionToken); err != nil {
		if errors.Is(err, applicationauth.ErrRecoveryInvalidRequest) || errors.Is(err, ports.ErrAuthTokenInvalid) {
			data.Error = "That verification link is invalid or expired."
			h.render(w, http.StatusUnprocessableEntity, "verify-email", data)
			return
		}
		h.renderError(w, http.StatusServiceUnavailable, "Verification unavailable", "We could not verify this email.")
		return
	}
	data.ActionToken = ""
	data.Notice = "Your email has been verified."
	h.render(w, http.StatusOK, "verify-email", data)
}

func (h *Handler) requestEmailVerification(w http.ResponseWriter, r *http.Request) {
	if h.recoveryService == nil || h.sessions == nil || r.Method != http.MethodPost {
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
	if err := h.recoveryService.RequestEmailVerification(r.Context(), session.UserID); err != nil {
		if errors.Is(err, applicationauth.ErrRecoveryInvalidRequest) {
			h.renderError(w, http.StatusUnprocessableEntity, "Verification unavailable", "This account cannot request verification.")
			return
		}
		h.renderError(w, http.StatusServiceUnavailable, "Verification unavailable", "Verification email delivery is not configured yet.")
		return
	}
	h.redirect(w, r, "/account/sessions?verification=sent")
}

func recoveryPageData(h *Handler, w http.ResponseWriter, r *http.Request, title string) pageData {
	return pageData{Brand: identity.Name, Tagline: identity.Tagline, Description: identity.Description, Title: title + " · " + identity.Name, CSRFToken: h.ensureCSRFCookie(w, r)}
}
