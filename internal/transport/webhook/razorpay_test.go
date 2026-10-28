package webhook

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wecratfs/commerce/internal/adapters/razorpay"
	"github.com/wecratfs/commerce/internal/application/payment"
	"github.com/wecratfs/commerce/internal/ports"
)

type webhookRepository struct {
	events map[string]bool
	last   ports.PaymentWebhook
}

func (r *webhookRepository) ProcessWebhook(_ context.Context, event ports.PaymentWebhook, _ []byte) (bool, error) {
	r.last = event
	if r.events[event.EventID] {
		return false, nil
	}
	r.events[event.EventID] = true
	return true, nil
}

func TestRazorpayDeliveryAuthenticatesRawBodyAndUsesEventHeader(t *testing.T) {
	const raw = `{"entity":"event","event":"payment.captured","payload":{"payment":{"entity":{"id":"pay_123","order_id":"order_123","amount":12900,"currency":"INR","status":"captured"}}}}`
	mac := hmac.New(sha256.New, []byte("webhook-secret"))
	_, _ = mac.Write([]byte(raw))
	signature := hex.EncodeToString(mac.Sum(nil))
	repository := &webhookRepository{events: map[string]bool{}}
	handler := NewRazorpayHandler(payment.NewService(razorpay.NewGateway("key", "secret", "webhook-secret", ""), repository))
	for _, tc := range []struct {
		body, signature string
		status          int
		processed       bool
	}{
		{raw, "invalid", http.StatusUnauthorized, false},
		{raw + " ", signature, http.StatusUnauthorized, false},
		{raw, signature, http.StatusOK, true},
		{raw, signature, http.StatusOK, false},
	} {
		req := httptest.NewRequest(http.MethodPost, "/webhooks/razorpay", strings.NewReader(tc.body))
		req.Header.Set("X-Razorpay-Signature", tc.signature)
		req.Header.Set("X-Razorpay-Event-Id", "evt_provider_123")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, req)
		if response.Code != tc.status {
			t.Fatalf("status = %d, body = %s", response.Code, response.Body)
		}
		if tc.status == http.StatusUnauthorized && len(repository.events) != 0 {
			t.Fatal("unverified delivery reached repository")
		}
		if tc.status == http.StatusOK {
			want := `"processed":false`
			if tc.processed {
				want = `"processed":true`
			}
			if !strings.Contains(response.Body.String(), want) || repository.last.EventID != "evt_provider_123" {
				t.Fatalf("delivery result = %s, event = %+v", response.Body, repository.last)
			}
		}
	}
}
