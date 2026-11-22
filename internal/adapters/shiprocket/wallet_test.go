package shiprocket

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWalletFundedRequiresReadablePositiveBalance(t *testing.T) {
	for _, tc := range []struct {
		name, body    string
		funded, fails bool
	}{
		{"positive", `{"data":{"balance_amount":"200.05"}}`, true, false},
		{"zero", `{"data":{"balance_amount":"0.00"}}`, false, false},
		{"negative", `{"data":{"balance_amount":-40.50}}`, false, false},
		{"missing", `{"data":{}}`, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/v1/external/auth/login" && r.Method == http.MethodPost {
					fmt.Fprint(w, `{"token":"fixture"}`)
					return
				}
				if r.URL.Path != "/v1/external/account/details/wallet-balance" || r.Method != http.MethodGet || r.Header.Get("Authorization") != "Bearer fixture" {
					t.Errorf("unexpected wallet request: %s %s", r.Method, r.URL.Path)
					w.WriteHeader(404)
					return
				}
				fmt.Fprint(w, tc.body)
			}))
			defer server.Close()
			funded, err := New(server.URL, "api@example.invalid", "fixture").WalletFunded(context.Background())
			if funded != tc.funded || (err != nil) != tc.fails {
				t.Fatalf("funded=%t err=%v", funded, err)
			}
		})
	}
}
