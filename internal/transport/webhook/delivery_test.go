package webhook

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDeliveryCallbackAuthenticatesAndOnlyQueuesMatchedIdentifiers(t *testing.T) {
	const secret = "fixture-secret-for-delivery-callback-only"
	const event = `{"sr_order_id":42,"order_id":"WC-test","awb":"TEST-WAYBILL","shipment_status":"DELIVERED","scans":[]}`
	for _, test := range []struct {
		name, token, media, body string
		status                   int
		fails                    bool
	}{
		{"valid", secret, "application/json", event, 200, false},
		{"bad token", "wrong", "application/json", event, 401, false},
		{"unexpected document", secret, "text/plain", event, 415, false},
		{"missing provider order", secret, "application/json", `{"order_id":"WC-test"}`, 400, false},
		{"oversize", secret, "application/json", strings.Repeat(" ", 256<<10) + event, 413, false},
		{"storage failed", secret, "application/json", event, 503, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			calls := 0
			h := DeliveryCallback(secret, func(_ context.Context, id int64, ref, awb string) error {
				calls++
				if id != 42 || ref != "WC-test" || awb != "TEST-WAYBILL" {
					t.Fatalf("incorrect hint %d %s %s", id, ref, awb)
				}
				if test.fails {
					return errors.New("unavailable")
				}
				return nil
			})
			r := httptest.NewRequest(http.MethodPost, DeliveryCallbackPath, strings.NewReader(test.body))
			r.Header.Set("X-API-Key", test.token)
			r.Header.Set("Content-Type", test.media)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != test.status || (calls == 1) != (test.status == 200 || test.fails) {
				t.Fatalf("status=%d schedule calls=%d", w.Code, calls)
			}
		})
	}
}
