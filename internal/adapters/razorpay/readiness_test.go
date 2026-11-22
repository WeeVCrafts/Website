package razorpay

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCheckoutReadinessChecksOrdersWithoutWriting(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, secret, _ := r.BasicAuth()
		if r.Method != http.MethodGet || r.URL.Path != "/v1/orders" || r.URL.Query().Get("count") != "1" || user != "rzp_live_fixture" || secret != "fixture" {
			t.Errorf("unexpected checkout readiness request: %s %s", r.Method, r.URL.RequestURI())
			w.WriteHeader(401)
			return
		}
		fmt.Fprint(w, `{"entity":"collection","count":0,"items":[]}`)
	}))
	defer server.Close()
	if err := NewGateway("rzp_live_fixture", "fixture", "", server.URL).CheckOrdersAccess(context.Background()); err != nil {
		t.Fatal(err)
	}
}
