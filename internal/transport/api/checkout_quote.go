package api

import (
	"errors"
	"net/http"

	"github.com/wecratfs/commerce/internal/ports"
)

func (h *Handler) CheckoutQuote(w http.ResponseWriter, r *http.Request) {
	session, ok := h.session(r)
	if r.Method != http.MethodGet || !ok || h.cart == nil || h.orders == nil {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "Sign in to preview your checkout.")
		return
	}
	cart, err := h.cart.GetOrCreate(r.Context(), session.UserID, "")
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "CART_UNAVAILABLE", "Your cart could not be loaded.")
		return
	}
	quote, err := h.orders.Quote(r.Context(), session.UserID, cart.ID, r.URL.Query().Get("coupon_code"))
	if err != nil {
		switch {
		case errors.Is(err, ports.ErrCouponInvalid):
			writeError(w, http.StatusUnprocessableEntity, "COUPON_INVALID", "This coupon is invalid, expired, or unavailable for your items.")
		case errors.Is(err, ports.ErrCartNotFound):
			writeError(w, http.StatusUnprocessableEntity, "CART_EMPTY", "Your cart is empty or has expired. Add items before checkout.")
		case errors.Is(err, ports.ErrInsufficientStock), errors.Is(err, ports.ErrInvalidState):
			writeError(w, http.StatusUnprocessableEntity, "ITEMS_UNAVAILABLE", "Some items are unavailable or exceed current stock. Update your cart.")
		default:
			writeError(w, http.StatusServiceUnavailable, "QUOTE_UNAVAILABLE", "Checkout could not be loaded. Please try again.")
		}
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{"data": quote})
}
