package webhook

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/wecratfs/commerce/internal/ports"
)

func TestResendWebhookRejectsTamperingAndSkipsUntrackedMail(t *testing.T) {
	key := []byte("test-signing-key-not-a-real-secret")
	const deliveryID = "ef4c2c91-940e-4379-91f4-86a1d2d3b588"
	const providerID = "bbdaf38b-2d5e-4b78-9dea-2ccf58be6b3b"
	var events []ports.EmailDeliveryEvent
	handler := ResendCallback("whsec_"+base64.StdEncoding.EncodeToString(key), func(_ context.Context, event ports.EmailDeliveryEvent) error {
		events = append(events, event)
		return nil
	})
	now := time.Now().UTC()
	body := fmt.Sprintf(`{"type":"email.bounced","created_at":%q,"data":{"email_id":%q,"tags":{"weevcrafts_delivery":%q},"to":["private@example.invalid"]}}`, now.Format(time.RFC3339Nano), providerID, deliveryID)
	for _, tc := range []struct {
		name, signed, received string
		stamp                  time.Time
		status                 int
	}{
		{"valid", body, body, now, http.StatusOK},
		{"tampered", body, body + " ", now, http.StatusUnauthorized},
		{"expired", body, body, now.Add(-6 * time.Minute), http.StatusUnauthorized},
		{"untracked", `{"type":"email.delivered"}`, `{"type":"email.delivered"}`, now, http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stamp := fmt.Sprint(tc.stamp.Unix())
			mac := hmac.New(sha256.New, key)
			fmt.Fprint(mac, "msg_fixture."+stamp+"."+tc.signed)
			r := httptest.NewRequest(http.MethodPost, ResendCallbackPath, strings.NewReader(tc.received))
			r.Header.Set("svix-id", "msg_fixture")
			r.Header.Set("svix-timestamp", stamp)
			r.Header.Set("svix-signature", "v1,"+base64.StdEncoding.EncodeToString(mac.Sum(nil)))
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)
			if w.Code != tc.status {
				t.Fatalf("status %d, want %d", w.Code, tc.status)
			}
		})
	}
	if len(events) != 1 || events[0].State != "bounced" || events[0].DeliveryID != deliveryID || events[0].ProviderID != providerID {
		t.Fatalf("unexpected recorded events: %+v", events)
	}
}
