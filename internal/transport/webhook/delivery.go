package webhook

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const DeliveryCallbackPath = "/webhooks/delivery"

// DeliveryCallback accepts a tracking hint. The reconciler reads the status
// from the provider's authenticated order endpoint before updating fulfillment.
func DeliveryCallback(secret string, schedule func(context.Context, int64, string, string) error) http.Handler {
	expected := sha256.Sum256([]byte(secret))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", "POST")
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		keys := r.Header.Values("X-API-Key")
		if len(keys) != 1 || len(secret) < 32 || schedule == nil {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		provided := sha256.Sum256([]byte(keys[0]))
		if subtle.ConstantTimeCompare(provided[:], expected[:]) != 1 {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		contentType, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if contentType != "application/json" || r.Header.Get("Content-Encoding") != "" {
			w.WriteHeader(http.StatusUnsupportedMediaType)
			return
		}
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 256<<10))
		if err != nil {
			w.WriteHeader(http.StatusRequestEntityTooLarge)
			return
		}
		var event struct {
			OrderID   json.Number `json:"sr_order_id"`
			Reference string      `json:"order_id"`
			AWB       string      `json:"awb"`
		}
		if json.Unmarshal(body, &event) != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		id, err := strconv.ParseInt(event.OrderID.String(), 10, 64)
		if err != nil || id <= 0 || strings.TrimSpace(event.Reference) == "" || len(event.Reference) > 120 || len(event.AWB) > 120 {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		if schedule(ctx, id, event.Reference, event.AWB) != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
}
