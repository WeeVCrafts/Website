package web

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestExactRupeeConversion(t *testing.T) {
	for raw, want := range map[string]int64{"899.50": 89950, "0.01": 1, "0": 0, "10.5": 1050, " 1200 ": 120000, "92233720368547758.07": 9223372036854775807} {
		got, err := rupeesToPaise(raw)
		if err != nil || got != want {
			t.Errorf("%q => %d, %v; want %d", raw, got, err, want)
		}
	}
	for _, raw := range []string{"", "1.001", "-1", "1e3", "1,000", ".50", "2.", "NaN", "92233720368547758.08"} {
		if _, err := rupeesToPaise(raw); err == nil {
			t.Errorf("accepted invalid money %q", raw)
		}
	}
}

func TestMoneyFormCompatibilityAndExplicitRupeePriority(t *testing.T) {
	for _, test := range []struct {
		body    string
		want    int64
		invalid bool
	}{{"price_rupees=899.50", 89950, false}, {"price_cents=89950", 89950, false}, {"price_rupees=0.01&price_cents=9900", 1, false}, {"price_rupees=&price_cents=9900", 0, true}} {
		r := httptest.NewRequest("POST", "/seller-admin/products", strings.NewReader(test.body))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		got, err := moneyFromForm(r, "price_cents")
		if (err != nil) != test.invalid || !test.invalid && got != test.want {
			t.Errorf("%s: got %d %v", test.body, got, err)
		}
	}
}
