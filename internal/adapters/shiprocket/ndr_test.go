package shiprocket

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wecratfs/commerce/internal/ports"
)

func TestNDRRecognizesExplicitProviderAcknowledgements(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		want       error
	}{
		{"success boolean", `{"success":true}`, nil},
		{"status boolean", `{"status":true}`, nil},
		{"documented acceptance", `{"status":"Data Updated Sucessfully"}`, nil},
		{"corrected acceptance spelling", `{"status":"Data Updated Successfully"}`, nil},
		{"explicit rejection", `{"success":false,"status":"Data Updated Successfully"}`, ports.ErrShippingRejected},
		{"status rejection", `{"status":false}`, ports.ErrShippingRejected},
		{"unknown response", `{"status":"processing"}`, ports.ErrShippingPending},
		{"incomplete response", `{}`, ports.ErrShippingPending},
		{"null response is uncertain", `{"status":null}`, ports.ErrShippingPending},
	} {
		t.Run(tc.name, func(t *testing.T) {
			posts := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/v1/external/auth/login" {
					w.Write([]byte(`{"token":"fixture-only"}`))
					return
				}
				if r.URL.Path != "/v1/external/ndr/TEST-AWB/action" || r.Method != http.MethodPost || r.Header.Get("Authorization") != "Bearer fixture-only" {
					t.Error("incorrect NDR request")
				}
				var input map[string]string
				if err := json.NewDecoder(r.Body).Decode(&input); err != nil || input["action"] != "re-attempt" || input["deferred_date"] != "2026-11-18" || input["comments"] != "Customer confirmed availability" {
					t.Error("incorrect NDR action payload")
				}
				posts++
				w.WriteHeader(http.StatusAccepted)
				w.Write([]byte(tc.body))
			}))
			defer server.Close()
			err := New(server.URL, "qa@example.invalid", "fixture-only").ActNDR(context.Background(), ports.Shipment{AWB: "TEST-AWB"}, "re-attempt", "2026-11-18", "Customer confirmed availability")
			if !errors.Is(err, tc.want) || posts != 1 {
				t.Fatalf("error=%v want=%v requests=%d", err, tc.want, posts)
			}
		})
	}
}
