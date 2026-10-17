package api

import (
	"errors"
	"net/http"
	"strings"

	applicationcommerce "github.com/wecratfs/commerce/internal/application/commerce"
	"github.com/wecratfs/commerce/internal/ports"
)

func (h *Handler) CheckoutIntent(w http.ResponseWriter, r *http.Request) {
	session, ok := h.session(r)
	if r.Method != http.MethodPost || !ok || h.cart == nil || !h.validCSRF(r, session) {
		writeError(w, 403, "FORBIDDEN", "Sign in again before starting checkout.")
		return
	}
	var input struct {
		VariantID int64  `json:"variant_id"`
		Quantity  int64  `json:"quantity"`
		Key       string `json:"idempotency_key"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	if key := strings.TrimSpace(r.Header.Get("Idempotency-Key")); key != "" {
		input.Key = key
	}
	intent, err := h.cart.CreateCheckoutIntent(r.Context(), session.UserID, input.VariantID, input.Quantity, input.Key)
	if err != nil {
		status := 422
		code, message := "ITEMS_UNAVAILABLE", "This option is unavailable or exceeds current stock. Check your selection."
		if errors.Is(err, ports.ErrForbidden) {
			status = 403
			code = "FORBIDDEN"
			message = "Use a customer account to start checkout."
		} else if errors.Is(err, ports.ErrIdempotencyConflict) {
			status = 409
			code = "IDEMPOTENCY_CONFLICT"
			message = "This purchase request has a different selection. Start Buy now again."
		} else if errors.Is(err, applicationcommerce.ErrInvalidQuantity) {
			message = "Choose a quantity from 1 to 99."
		}
		writeError(w, status, code, message)
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	writeJSON(w, 200, map[string]any{"data": map[string]any{"id": intent.ID, "cart_id": intent.CartID, "expires_at": intent.ExpiresAt, "checkout_url": "/checkout?intent=" + intent.ID}})
}
