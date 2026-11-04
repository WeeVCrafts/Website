package razorpay

import (
	"context"
	"errors"
	"fmt"
	"github.com/wecratfs/commerce/internal/ports"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPaymentLookupIsReadOnlyAndValidatesEveryAttempt(t *testing.T) {
	for _, bad := range []bool{false, true} {
		t.Run(fmt.Sprint(bad), func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != http.MethodGet || r.URL.Path != "/v1/orders/order_fixture/payments" {
					t.Errorf("unexpected provider mutation %s %s", r.Method, r.URL.Path)
				}
				if key, secret, ok := r.BasicAuth(); !ok || key != "test_key" || secret != "test_secret" {
					t.Error("provider authentication missing")
				}
				amount := 1000
				if bad {
					amount = 1001
				}
				fmt.Fprintf(w, `{"items":[{"id":"pay_failed","order_id":"order_fixture","amount":1000,"currency":"INR","status":"failed"},{"id":"pay_captured","order_id":"order_fixture","amount":%d,"currency":"INR","status":"captured"}]}`, amount)
			}))
			defer server.Close()
			gateway := NewGateway("test_key", "test_secret", "test_webhook", server.URL)
			events, err := gateway.LookupPayments(context.Background(), ports.PaymentOrderRecord{ProviderOrderID: "order_fixture", AmountCents: 1000, Currency: "INR"})
			if bad {
				if !errors.Is(err, ports.ErrPaymentMismatch) {
					t.Fatalf("mismatch accepted: %v", err)
				}
				return
			}
			if err != nil || len(events) != 2 || events[1].EventType != "payment.captured" || events[0].EventID == events[1].EventID {
				t.Fatalf("events=%+v err=%v", events, err)
			}
			replay, err := gateway.LookupPayments(context.Background(), ports.PaymentOrderRecord{ProviderOrderID: "order_fixture", AmountCents: 1000, Currency: "INR"})
			if err != nil || replay[1].EventID != events[1].EventID || calls != 2 {
				t.Fatal("unstable reconciliation identity")
			}
		})
	}
}
