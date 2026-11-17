package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/wecratfs/commerce/internal/ports"
)

type deliveryProbe struct {
	calls     int
	pin, slug string
	variant   int64
	quantity  int
}

func (p *deliveryProbe) ProductDelivery(_ context.Context, slug string, variant int64, quantity int, pin string) (ports.ProductDeliveryResult, error) {
	p.calls++
	p.pin, p.slug, p.variant, p.quantity = pin, slug, variant, quantity
	return ports.ProductDeliveryResult{State: "available", TransitDays: 4}, nil
}

type deliveryLimit bool

func (d deliveryLimit) Allow(context.Context, string, int, time.Duration) (bool, error) {
	return bool(d), nil
}

func TestProductPINCheckValidatesBeforeCarrierLookup(t *testing.T) {
	for _, tc := range []struct {
		query         string
		allowed       bool
		status, calls int
	}{
		{"postal_code=500079&variant_id=7&quantity=2", true, 200, 1},
		{"postal_code=050079&variant_id=7&quantity=2", true, 400, 0},
		{"postal_code=500079&variant_id=7&quantity=0", true, 400, 0},
		{"postal_code=500079&variant_id=7&quantity=2", false, 429, 0},
	} {
		probe := &deliveryProbe{}
		mux := http.NewServeMux()
		mux.Handle("/api/v1/products/{slug}/delivery", ProductDelivery(probe, deliveryLimit(tc.allowed), func(*http.Request) string { return "fixture" }))
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/products/painting/delivery?"+tc.query, nil))
		if w.Code != tc.status || probe.calls != tc.calls {
			t.Fatalf("%s status=%d calls=%d", tc.query, w.Code, probe.calls)
		}
		if probe.calls > 0 && (probe.slug != "painting" || probe.pin != "500079" || probe.variant != 7 || probe.quantity != 2) {
			t.Fatal("selected parcel was not passed to delivery lookup")
		}
	}
}
