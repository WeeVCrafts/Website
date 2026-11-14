package razorpay

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wecratfs/commerce/internal/ports"
)

func TestOrderLookupUsesExactEncodedReceiptAndBoundedResult(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		want       int
		mismatch   bool
	}{
		{"one", `{"entity":"collection","items":[{"id":"order_1","receipt":"WC & 1","amount":1200,"currency":"INR"}]}`, 1, false},
		{"none", `{"entity":"collection","items":[]}`, 0, false},
		{"wrong receipt", `{"entity":"collection","items":[{"id":"order_1","receipt":"WC & 11","amount":1200,"currency":"INR"}]}`, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				key, secret, ok := r.BasicAuth()
				if r.Method != "GET" || r.URL.Path != "/v1/orders" || r.URL.Query().Get("receipt") != "WC & 1" || r.URL.Query().Get("count") != "2" || !ok || key != "test" || secret != "secret" {
					t.Errorf("unexpected lookup request")
				}
				fmt.Fprint(w, tc.body)
			}))
			defer server.Close()
			orders, err := NewGateway("test", "secret", "", server.URL).LookupOrdersByReceipt(t.Context(), "WC & 1")
			if len(orders) != tc.want || errors.Is(err, ports.ErrPaymentMismatch) != tc.mismatch || (!tc.mismatch && err != nil) {
				t.Fatalf("orders=%+v err=%v", orders, err)
			}
		})
	}
}
