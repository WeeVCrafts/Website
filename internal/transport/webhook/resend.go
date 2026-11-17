package webhook

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/wecratfs/commerce/internal/ports"
)

const ResendCallbackPath = "/webhooks/resend"

var resendDeliveryStates = map[string]string{
	"email.sent": "accepted", "email.delivery_delayed": "delayed", "email.delivered": "delivered",
	"email.bounced": "bounced", "email.failed": "failed", "email.suppressed": "suppressed", "email.complained": "complained",
}

func ResendCallback(secret string, record func(context.Context, ports.EmailDeliveryEvent) error) http.Handler {
	key, _ := base64.StdEncoding.DecodeString(strings.TrimPrefix(secret, "whsec_"))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", "POST")
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 64<<10))
		if err != nil {
			w.WriteHeader(http.StatusRequestEntityTooLarge)
			return
		}
		eventID := r.Header.Get("svix-id")
		stamp := r.Header.Get("svix-timestamp")
		if len(key) < 16 || record == nil || !validResendSignature(key, eventID, stamp, r.Header.Get("svix-signature"), body) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		var payload struct {
			Type string    `json:"type"`
			At   time.Time `json:"created_at"`
			Data struct {
				EmailID string            `json:"email_id"`
				Tags    map[string]string `json:"tags"`
			} `json:"data"`
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		state := resendDeliveryStates[payload.Type]
		tag := payload.Data.Tags["weevcrafts_delivery"]
		if state == "" || tag == "" {
			w.WriteHeader(http.StatusOK)
			return
		}
		delivery, dErr := uuid.Parse(tag)
		provider, pErr := uuid.Parse(payload.Data.EmailID)
		if dErr != nil || pErr != nil || delivery == uuid.Nil || provider == uuid.Nil || payload.At.IsZero() || payload.At.After(time.Now().Add(5*time.Minute)) {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		if err := record(ctx, ports.EmailDeliveryEvent{ID: eventID, DeliveryID: delivery.String(), ProviderID: provider.String(), State: state, At: payload.At}); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
}

func validResendSignature(key []byte, eventID, stamp, signatures string, body []byte) bool {
	seconds, err := strconv.ParseInt(stamp, 10, 64)
	now := time.Now().Unix()
	if err != nil || seconds < now-300 || seconds > now+300 || len(eventID) == 0 || len(eventID) > 128 || strings.ContainsAny(eventID, ". \t\r\n") {
		return false
	}
	mac := hmac.New(sha256.New, key)
	_, _ = io.WriteString(mac, eventID+"."+stamp+".")
	_, _ = mac.Write(body)
	expected := mac.Sum(nil)
	for _, versioned := range strings.Fields(signatures) {
		version, signature, ok := strings.Cut(versioned, ",")
		if !ok || version != "v1" {
			continue
		}
		provided, err := base64.StdEncoding.DecodeString(signature)
		if err == nil && hmac.Equal(provided, expected) {
			return true
		}
	}
	return false
}
