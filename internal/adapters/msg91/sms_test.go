package msg91

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wecratfs/commerce/internal/ports"
)

func testSender(t *testing.T, handler http.HandlerFunc) *Sender {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	s, err := New("fixture-only-authentication-key", "WEEVCR", map[string]ports.SMSTemplate{"customer_order": {ID: "0123456789abcdef01234567", DLTID: "1234567890123456789", Variables: map[string]string{"VAR1": "reference", "VAR2": "status"}}})
	if err != nil {
		t.Fatal(err)
	}
	s.endpoint = server.URL
	return s
}

func TestMSG91SendsOnlyMappedTemplateAndInternationalAccountNumber(t *testing.T) {
	s := testSender(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.Header.Get("authkey") != "fixture-only-authentication-key" {
			t.Error("missing private server authentication")
		}
		var body struct {
			Template   string              `json:"template_id"`
			ShortURL   string              `json:"short_url"`
			Recipients []map[string]string `json:"recipients"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body.Template != "0123456789abcdef01234567" || body.ShortURL != "0" || len(body.Recipients) != 1 || body.Recipients[0]["mobiles"] != "919876543210" || body.Recipients[0]["VAR1"] != "WC-123" || body.Recipients[0]["VAR2"] != "Order confirmed" || len(body.Recipients[0]) != 3 {
			t.Error("incorrect transactional template payload")
		}
		w.Write([]byte(`{"type":"success","message":"0123456789abcdef01234567"}`))
	})
	r, err := s.SendTransactionalSMS(context.Background(), ports.SMSJob{TemplateKey: "customer_order", Phone: "+91 9876543210", Reference: "WC-123", Update: "Order confirmed"})
	if err != nil || r.Outcome != "accepted" || r.ProviderID == "" {
		t.Fatalf("acceptance %+v %v", r, err)
	}
}

func TestMSG91NeverLeaksProviderBodiesAndClassifiesUncertainty(t *testing.T) {
	for _, tc := range []struct {
		name          string
		status        int
		body, outcome string
	}{
		{"explicit rejection", 400, `{"type":"error","message":"private phone and secret details"}`, "rejected"},
		{"provider failure", 500, `{"type":"error","message":"private phone and secret details"}`, "uncertain"},
		{"lost response", 200, `{"type":"success","message":`, "uncertain"},
		{"redirect", 302, `redirect`, "uncertain"},
		{"empty acceptance", 200, `{"type":"success"}`, "uncertain"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := testSender(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(tc.status); w.Write([]byte(tc.body)) })
			r, err := s.SendTransactionalSMS(context.Background(), ports.SMSJob{TemplateKey: "customer_order", Phone: "9876543210", Reference: "WC-123", Update: "Order confirmed"})
			if err == nil || r.Outcome != tc.outcome || strings.Contains(err.Error(), "private phone") {
				t.Fatalf("result %+v %v", r, err)
			}
		})
	}
}

func TestMSG91InvalidRecipientOrTemplateNeverCallsProvider(t *testing.T) {
	called := false
	s := testSender(t, func(w http.ResponseWriter, r *http.Request) { called = true })
	for _, job := range []ports.SMSJob{{TemplateKey: "unknown", Phone: "9876543210", Reference: "WC-123", Update: "Order confirmed"}, {TemplateKey: "customer_order", Phone: "+12025550123", Reference: "WC-123", Update: "Order confirmed"}, {TemplateKey: "customer_order", Phone: "9876543210", Reference: "WC-123", Update: "body\nspoof"}} {
		if r, err := s.SendTransactionalSMS(context.Background(), job); err == nil || r.Outcome != "skipped" {
			t.Fatal("invalid SMS accepted")
		}
	}
	if called {
		t.Fatal("invalid request contacted MSG91")
	}
}
