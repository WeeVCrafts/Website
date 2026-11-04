package web

import (
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestInventoryAdjustmentDirectionAndLegacyDelta(t *testing.T) {
	for _, test := range []struct {
		name    string
		fields  url.Values
		want    int
		invalid bool
	}{
		{"add", url.Values{"adjustment_type": {"add"}, "quantity": {"7"}}, 7, false},
		{"remove", url.Values{"adjustment_type": {"remove"}, "quantity": {"7"}}, -7, false},
		{"legacy", url.Values{"delta": {"-4"}}, -4, false},
		{"negative", url.Values{"adjustment_type": {"remove"}, "quantity": {"-7"}}, 0, true},
		{"unknown direction", url.Values{"adjustment_type": {"set"}, "quantity": {"7"}}, 0, true},
		{"fraction", url.Values{"adjustment_type": {"add"}, "quantity": {"1.5"}}, 0, true},
		{"too large", url.Values{"adjustment_type": {"add"}, "quantity": {"100001"}}, 0, true},
		{"explicit wins", url.Values{"adjustment_type": {"remove"}, "quantity": {"7"}, "delta": {"999"}}, -7, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			r := httptest.NewRequest("POST", "/seller/inventory/adjust", strings.NewReader(test.fields.Encode()))
			r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			got, err := inventoryDelta(r)
			if (err != nil) != test.invalid || got != test.want {
				t.Fatalf("delta=%d error=%v", got, err)
			}
		})
	}
}

func TestSellerOrderReturnStateCannotRedirectOffsite(t *testing.T) {
	r := httptest.NewRequest("POST", "/seller/orders/fulfill", nil)
	r.Form = url.Values{"return_query": {"blue & gold"}, "return_tab": {"packing"}, "return_page": {"3"}, "return_url": {"https://example.invalid"}}
	u, err := url.Parse(sellerOrderReturnURL(r, "WC-1", "Saved"))
	if err != nil || u.Host != "" || u.Path != "/seller-admin/orders/WC-1" || u.Query().Get("q") != "blue & gold" || u.Query().Get("page") != "3" {
		t.Fatalf("bad return state: %s", u)
	}
}
