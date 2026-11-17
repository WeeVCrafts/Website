package handlers

import (
	"net/url"
	"testing"
)

func TestSetupTaxPercentageUsesExactHundredths(t *testing.T) {
	for _, c := range []struct {
		percent, legacy string
		want            int
		bad             bool
	}{
		{"0", "", 0, false}, {"5", "", 500, false}, {"5.50", "", 550, false}, {"0.01", "", 1, false}, {"100.00", "", 10000, false},
		{"5.555", "", 0, true}, {"-1", "", 0, true}, {"100.01", "", 0, true}, {"", "", 0, true}, {"5", "500", 500, false}, {"5", "600", 0, true},
	} {
		t.Run(c.percent+"/"+c.legacy, func(t *testing.T) {
			v := url.Values{"tax_rate_percent": {c.percent}}
			if c.legacy != "" {
				v.Set("rate_basis_points", c.legacy)
			}
			n, err := parseTaxPercent(v)
			if (err != nil) != c.bad || !c.bad && n != c.want {
				t.Fatalf("got %d, %v", n, err)
			}
		})
	}
	if n, err := parseTaxPercent(url.Values{"rate_basis_points": {"1250"}}); err != nil || n != 1250 {
		t.Fatalf("legacy rate: %d %v", n, err)
	}
}

func TestSetupInvoiceValidationAndURLPrivacy(t *testing.T) {
	f := url.Values{"seller_id": {"2"}, "legal_name": {"Fixture Seller"}, "address": {"Fixture address Hyderabad"}, "state_code": {"TG"}, "prefix": {"TEST"}, "registration": {"registered"}, "reason": {"Checked fixture documents"}}
	e := validateSetup("save_invoice_profile", f)
	if e["state_code"] == "" || e["gstin"] == "" {
		t.Fatalf("missing state/GSTIN validation: %v", e)
	}
	f.Set("state_code", "36")
	f.Set("registration", "unregistered")
	if e = validateSetup("save_invoice_profile", f); len(e) != 0 {
		t.Fatalf("valid invoice: %v", e)
	}
	f.Set("setup_context", "seller_id=2&variant_id=1&address=private&gstin=private&tab=operations")
	q := setupPostContext(f)
	if q.Get("seller_id") != "2" || q.Get("tab") != "setup" || q.Has("address") || q.Has("gstin") {
		t.Fatalf("unsafe context: %v", q)
	}
}
