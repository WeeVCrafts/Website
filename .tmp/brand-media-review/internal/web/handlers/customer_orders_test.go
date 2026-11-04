package handlers

import (
	"net/url"
	"testing"
)

func TestCustomerOrderReturnStateIsLocalAndBounded(t *testing.T) {
	for input, want := range map[string]string{
		"": "/orders", "https://example.invalid/orders?page=3": "/orders", "//example.invalid/orders": "/orders",
		"/orders?status=delivered&page=2&untrusted=value": "/orders?page=2&status=delivered",
		"/orders?status=unknown&page=-1":                  "/orders", "/account": "/orders",
	} {
		if got := customerOrderListURL(input); got != want {
			t.Errorf("%q: got %q want %q", input, got, want)
		}
	}
	got := customerOrderDetailsURL("WC-CUSTOMER", url.Values{"status": {"active"}, "page": {"2"}})
	u, err := url.Parse(got)
	if err != nil || u.Path != "/orders/WC-CUSTOMER" || u.Query().Get("from") != "/orders?page=2&status=active" {
		t.Fatalf("detail state=%s err=%v", got, err)
	}
}
