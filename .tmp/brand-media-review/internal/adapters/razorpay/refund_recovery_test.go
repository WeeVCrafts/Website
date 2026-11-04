package razorpay

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wecratfs/commerce/internal/ports"
)

func TestRefundWebhookRequiresOriginalSignature(t *testing.T) {
	raw := []byte(`{"event":"refund.processed","payload":{"refund":{"entity":{"id":"rfnd_fixture","payment_id":"pay_fixture","receipt":"refund-7","amount":1000,"currency":"INR","status":"processed"}}}}`)
	mac := hmac.New(sha256.New, []byte("webhook-secret"))
	mac.Write(raw)
	signature := hex.EncodeToString(mac.Sum(nil))
	gateway := NewGateway("test", "secret", "webhook-secret", "")
	event, err := gateway.VerifyWebhook(context.Background(), raw, signature)
	if err != nil || event.ProviderRefundID != "rfnd_fixture" || event.RefundReceipt != "refund-7" {
		t.Fatalf("event=%+v err=%v", event, err)
	}
	if _, err := gateway.VerifyWebhook(context.Background(), append(raw, ' '), signature); err != ErrInvalidSignature {
		t.Fatal("altered refund webhook accepted")
	}
}

func TestRefundRecoveryNeverPostsAndFindsOriginalReceipt(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != http.MethodGet || r.URL.Path != "/v1/payments/pay_fixture/refunds" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(400)
			return
		}
		if r.URL.Query().Get("count") != "100" {
			t.Error("missing page limit")
		}
		fmt.Fprint(w, `{"items":[{"id":"rfnd_other","receipt":"unrelated"},{"id":"rfnd_original","payment_id":"pay_fixture","receipt":"refund-7","amount":1000,"currency":"INR","status":"processed"}]}`)
	}))
	defer server.Close()
	refund, err := NewGateway("test", "secret", "webhook", server.URL).LookupRefund(context.Background(), ports.PaymentRefundRecord{ID: 7, ProviderPaymentID: "pay_fixture"})
	if err != nil || refund.ProviderRefundID != "rfnd_original" || calls != 1 {
		t.Fatalf("refund=%+v calls=%d error=%v", refund, calls, err)
	}
}

func TestRefundRecoveryRejectsAmbiguousReceipt(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"items":[{"id":"rfnd_one","receipt":"refund-7"},{"id":"rfnd_two","receipt":"refund-7"}]}`)
	}))
	defer server.Close()
	if _, err := NewGateway("test", "secret", "webhook", server.URL).LookupRefund(context.Background(), ports.PaymentRefundRecord{ID: 7, ProviderPaymentID: "pay_fixture"}); err == nil {
		t.Fatal("ambiguous refund accepted")
	}
}
