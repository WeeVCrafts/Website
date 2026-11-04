package handlers

import (
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestNextFulfillmentStatusFollowsOrder(t *testing.T) {
	tests := map[string]string{
		"pending":          "processing",
		"processing":       "packing",
		"packing":          "shipped",
		"shipped":          "out_for_delivery",
		"out_for_delivery": "delivered",
		"delivered":        "",
		"cancelled":        "",
	}
	for current, want := range tests {
		if got := nextFulfillmentStatus(current); got != want {
			t.Errorf("nextFulfillmentStatus(%q) = %q, want %q", current, got, want)
		}
	}
}

func TestReadPromotionFormDiscountTypes(t *testing.T) {
	tests := []struct {
		name, kind, amount string
		wantValue          int64
		wantError          bool
	}{
		{name: "percentage whole number", kind: "percentage", amount: "15", wantValue: 15},
		{name: "percentage decimal rejected", kind: "percentage", amount: "15.5", wantError: true},
		{name: "percentage capped at 100", kind: "percentage", amount: "101", wantError: true},
		{name: "fixed rupees with paise", kind: "fixed", amount: "12.50", wantValue: 1250},
		{name: "free shipping ignores amount", kind: "free_shipping", amount: "", wantValue: 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			form := url.Values{
				"name":           {"QA campaign"},
				"code":           {"QA-CODE"},
				"discount_type":  {test.kind},
				"discount_value": {test.amount},
			}
			request := httptest.NewRequest("POST", "/marketing/create", strings.NewReader(form.Encode()))
			request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			if err := request.ParseForm(); err != nil {
				t.Fatal(err)
			}
			got, err := readPromotionForm(request)
			if (err != nil) != test.wantError {
				t.Fatalf("readPromotionForm() error = %v, wantError %t", err, test.wantError)
			}
			if err == nil && got.value != test.wantValue {
				t.Fatalf("discount value = %d, want %d", got.value, test.wantValue)
			}
		})
	}
}

func TestReadPromotionFormTargeting(t *testing.T) {
	form := url.Values{
		"name":           {"Selected products"},
		"code":           {"SELECT10"},
		"discount_type":  {"percentage"},
		"discount_value": {"10"},
		"targeting_mode": {"selected"},
		"product_ids":    {"12", "12", "bad", "19"},
	}
	request := httptest.NewRequest("POST", "/marketing/create", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if err := request.ParseForm(); err != nil {
		t.Fatal(err)
	}
	got, err := readPromotionForm(request)
	if err != nil {
		t.Fatalf("readPromotionForm() error = %v", err)
	}
	if got.targetingMode != "selected" || len(got.productIDs) != 2 || got.productIDs[0] != 12 || got.productIDs[1] != 19 {
		t.Fatalf("targeting = %q, product IDs = %#v", got.targetingMode, got.productIDs)
	}
}

func TestReadPromotionFormRejectsUnknownTargeting(t *testing.T) {
	form := url.Values{"name": {"QA campaign"}, "code": {"QA-CODE"}, "discount_type": {"percentage"}, "discount_value": {"10"}, "targeting_mode": {"unknown"}}
	request := httptest.NewRequest("POST", "/marketing/create", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if err := request.ParseForm(); err != nil {
		t.Fatal(err)
	}
	if _, err := readPromotionForm(request); err == nil {
		t.Fatal("expected unknown targeting mode to fail")
	}
}
