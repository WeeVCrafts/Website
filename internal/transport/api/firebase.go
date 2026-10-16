package api

import (
	"crypto/subtle"
	"errors"
	applicationauth "github.com/wecratfs/commerce/internal/application/auth"
	"net/http"
	"net/url"
)

func (h *Handler) WithFirebase(service *applicationauth.FirebaseService) *Handler {
	h.firebase = service
	return h
}

func (h *Handler) FirebaseSession(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "private, no-store")
	if r.Method != http.MethodPost {
		writeError(w, 405, "METHOD_NOT_ALLOWED", "Only POST is supported.")
		return
	}
	if h.firebase == nil {
		writeError(w, 503, "FIREBASE_UNAVAILABLE", "Customer sign-in is temporarily unavailable.")
		return
	}
	if !h.validFirebaseCSRF(r) {
		writeError(w, 403, "CSRF_INVALID", "Reload the sign-in page and try again.")
		return
	}
	var in applicationauth.FirebaseInput
	if !decodeJSON(w, r, &in) {
		return
	}
	user, err := h.firebase.Login(r.Context(), in)
	if err != nil {
		switch {
		case errors.Is(err, applicationauth.ErrFirebaseLinkRequired):
			writeError(w, 409, "LINK_REQUIRED", "Confirm your existing WeevCrafts password to securely connect this account.")
		case errors.Is(err, applicationauth.ErrFirebaseTerms):
			writeError(w, 422, "TERMS_REQUIRED", "Accept the privacy notice and terms to create your account.")
		case errors.Is(err, applicationauth.ErrFirebaseRole):
			writeError(w, 403, "CUSTOMER_ONLY", "Use your assigned portal credentials. Google sign-in is available for customer accounts.")
		default:
			writeError(w, 401, "FIREBASE_INVALID", "Sign-in could not be verified. Please sign in again.")
		}
		return
	}
	session, err := h.sessions.Start(r.Context(), user, r.UserAgent(), "", applicationauth.DefaultSessionTTL)
	if err != nil {
		writeError(w, 503, "SESSION_UNAVAILABLE", "A secure session could not be started.")
		return
	}
	h.setSessionCookie(w, session)
	h.mergeGuestCart(r.Context(), w, r, user.ID)
	writeJSON(w, 200, map[string]any{"data": user, "csrf_token": session.CSRFToken, "redirect_to": applicationauth.RedirectDestination(user.Roles, in.Next)})
}

func (h *Handler) validFirebaseCSRF(r *http.Request) bool {
	// Login/registration CSRF applies before a customer has an application session.
	origin, err := url.Parse(r.Header.Get("Origin"))
	cookie, cookieErr := r.Cookie("wecratfs_csrf")
	csrf := r.Header.Get("X-CSRF-Token")
	valid := err == nil && origin.Host == r.Host && origin.Path == "" && origin.RawQuery == "" && (origin.Scheme == "https" || (!h.secureCookies && origin.Scheme == "http"))
	return cookieErr == nil && len(csrf) >= 32 && len(csrf) <= 128 && subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(csrf)) == 1 && valid
}

func (h *Handler) FirebaseVerificationRequest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "private, no-store")
	if r.Method != http.MethodPost {
		writeError(w, 405, "METHOD_NOT_ALLOWED", "Only POST is supported.")
		return
	}
	if h.firebase == nil || h.recovery == nil {
		writeError(w, 503, "VERIFICATION_UNAVAILABLE", "Verification is unavailable.")
		return
	}
	if !h.validFirebaseCSRF(r) {
		writeError(w, 403, "CSRF_INVALID", "Reload the sign-in page and try again.")
		return
	}
	var input struct {
		Token string `json:"id_token"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	proof, err := h.firebase.VerifyIdentity(r.Context(), input.Token)
	if err != nil {
		writeError(w, 401, "FIREBASE_INVALID", "Sign-in could not be verified.")
		return
	}
	if err = h.recovery.RequestFirebaseVerification(r.Context(), proof); err != nil {
		if errors.Is(err, applicationauth.ErrRecoveryInvalidRequest) {
			writeError(w, 403, "CUSTOMER_ONLY", "Customer verification is required.")
			return
		}
		writeError(w, 503, "VERIFICATION_UNAVAILABLE", "Verification email could not be sent. Please try again later.")
		return
	}
	writeJSON(w, 202, map[string]string{"status": "accepted"})
}
