package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/wecratfs/commerce/internal/ports"
)

func ProductDelivery(reader ports.ProductDeliveryReader, limiter ports.RateLimiter, clientAddress func(*http.Request) string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET is supported.")
			return
		}
		query := r.URL.Query()
		pin := strings.TrimSpace(query.Get("postal_code"))
		variant, variantErr := strconv.ParseInt(query.Get("variant_id"), 10, 64)
		quantity, quantityErr := strconv.Atoi(query.Get("quantity"))
		if len(pin) != 6 || pin[0] == '0' || strings.Trim(pin, "0123456789") != "" || variantErr != nil || variant <= 0 || quantityErr != nil || quantity < 1 || quantity > 50000 {
			writeError(w, http.StatusBadRequest, "INVALID_DELIVERY_INPUT", "Choose an in-stock option and quantity, and enter a six-digit Indian PIN code.")
			return
		}
		unavailable := func() {
			writeJSON(w, http.StatusOK, ports.ProductDeliveryResult{State: "unavailable"})
		}
		if reader == nil || limiter == nil {
			unavailable()
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 4*time.Second)
		defer cancel()
		for _, budget := range []struct {
			key   string
			limit int
		}{
			{"product-delivery:" + clientAddress(r), 20}, {"product-delivery-global", 120},
		} {
			ok, err := limiter.Allow(ctx, budget.key, budget.limit, time.Minute)
			if err != nil {
				unavailable()
				return
			}
			if !ok {
				w.Header().Set("Retry-After", "60")
				writeError(w, http.StatusTooManyRequests, "DELIVERY_RATE_LIMITED", "Too many delivery checks. Try again in a minute.")
				return
			}
		}
		result, err := reader.ProductDelivery(ctx, r.PathValue("slug"), variant, quantity, pin)
		if errors.Is(err, ports.ErrNotFound) {
			writeError(w, http.StatusNotFound, "PRODUCT_OPTION_NOT_FOUND", "This product option is unavailable.")
			return
		}
		if err != nil {
			unavailable()
			return
		}
		writeJSON(w, http.StatusOK, result)
	})
}
