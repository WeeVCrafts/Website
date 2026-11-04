package api

import (
	"errors"
	"net/http"
	"regexp"
	"strings"

	domaincommerce "github.com/wecratfs/commerce/internal/domain/commerce"
	"github.com/wecratfs/commerce/internal/domain/storefront"
	"github.com/wecratfs/commerce/internal/ports"
)

func (h *Handler) CheckoutQuote(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	session, ok := h.session(r)
	if r.Method != http.MethodGet || !ok || h.cart == nil || h.orders == nil {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "Sign in to preview your checkout.")
		return
	}
	if pin := strings.TrimSpace(r.URL.Query().Get("postal_code")); pin != "" {
		if !regexp.MustCompile(`^[1-9][0-9]{5}$`).MatchString(pin) {
			writeError(w, http.StatusUnprocessableEntity, "PIN_INVALID", "Enter a valid six-digit Indian PIN code.")
			return
		}
		if revision, exists := storefront.Current(r.Context()); exists && !revision.Config.Settings.Policy.Serves(pin) {
			writeError(w, http.StatusUnprocessableEntity, "PIN_UNSERVICEABLE", "Delivery is not currently available to this PIN code. Your cart is saved.")
			return
		}
	}
	var cart domaincommerce.Cart
	var err error
	if token := r.URL.Query().Get("intent"); token != "" {
		cart, err = h.cart.GetCheckoutCart(r.Context(), session.UserID, token)
	} else {
		cart, err = h.cart.GetOrCreate(r.Context(), session.UserID, "")
	}
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "CART_UNAVAILABLE", "This checkout is unavailable or expired. Return to the product and start again.")
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
